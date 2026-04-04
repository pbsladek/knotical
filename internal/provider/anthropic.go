package provider

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/pbsladek/knotical/internal/model"
)

type AnthropicProvider struct{ client anthropic.Client }

func NewAnthropicProvider(apiKey string, baseURL string, timeout time.Duration) AnthropicProvider {
	options := []anthropicoption.RequestOption{}
	if apiKey != "" {
		options = append(options, anthropicoption.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		options = append(options, anthropicoption.WithBaseURL(baseURL))
	}
	if timeout > 0 {
		options = append(options, anthropicoption.WithRequestTimeout(timeout))
	}
	return AnthropicProvider{client: anthropic.NewClient(options...)}
}

func (p AnthropicProvider) Name() string { return "anthropic" }

func (p AnthropicProvider) Complete(ctx context.Context, req Request) (model.CompletionResponse, error) {
	resp, err := p.client.Messages.New(ctx, buildAnthropicRequest(req))
	if err != nil {
		return model.CompletionResponse{}, err
	}
	return completionFromAnthropicMessage(resp), nil
}

func (p AnthropicProvider) Stream(ctx context.Context, req Request, emit func(model.StreamChunk) error) error {
	stream := p.client.Messages.NewStreaming(ctx, buildAnthropicRequest(req))
	var usage *model.TokenUsage
	for stream.Next() {
		event := stream.Current()
		if delta, ok := anthropicTextDelta(event); ok {
			if err := emit(model.StreamChunk{Delta: delta}); err != nil {
				return err
			}
		}
		if currentUsage := usageFromAnthropicStreamEvent(event.RawJSON()); currentUsage != nil {
			usage = currentUsage
		}
		if event.Type == "message_stop" && usage != nil {
			if err := emit(model.StreamChunk{Usage: usage, Done: true}); err != nil {
				return err
			}
		}
	}
	return stream.Err()
}

func (p AnthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	pager := p.client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{})
	models := []string{}
	for pager.Next() {
		model := pager.Current()
		if model.ID == "" {
			continue
		}
		models = append(models, model.ID)
	}
	if err := pager.Err(); err != nil {
		return nil, err
	}
	slices.Sort(models)
	return models, nil
}

func buildAnthropicRequest(req Request) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: maxTokens(req.MaxTokens, 4096),
		Messages:  toAnthropicMessages(req.Messages),
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	if req.Temperature != nil {
		params.Temperature = anthropic.Float(*req.Temperature)
	}
	if req.TopP != nil {
		params.TopP = anthropic.Float(*req.TopP)
	}
	return params
}

func completionFromAnthropicMessage(resp *anthropic.Message) model.CompletionResponse {
	var content strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			content.WriteString(text.Text)
		}
	}
	result := model.CompletionResponse{Content: content.String(), Model: string(resp.Model)}
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		result.Usage = &model.TokenUsage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		}
	}
	return result
}

func anthropicTextDelta(event anthropic.MessageStreamEventUnion) (string, bool) {
	switch value := event.AsAny().(type) {
	case anthropic.ContentBlockDeltaEvent:
		if delta, ok := value.Delta.AsAny().(anthropic.TextDelta); ok {
			return delta.Text, true
		}
	}
	return "", false
}

func toAnthropicMessages(messages []model.Message) []anthropic.MessageParam {
	result := []anthropic.MessageParam{}
	for _, msg := range messages {
		switch msg.Role {
		case model.RoleAssistant:
			result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		case model.RoleSystem:
			continue
		default:
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}
	return result
}

func usageFromAnthropicStreamEvent(raw string) *model.TokenUsage {
	if raw == "" {
		return nil
	}
	var payload struct {
		Type  string `json:"type"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	if payload.Type != "message_delta" {
		return nil
	}
	total := payload.Usage.InputTokens + payload.Usage.OutputTokens
	if total == 0 {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     payload.Usage.InputTokens,
		CompletionTokens: payload.Usage.OutputTokens,
		TotalTokens:      total,
	}
}
