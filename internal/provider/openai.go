package provider

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/openai/openai-go"
	openaioption "github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/pbsladek/knotical/internal/model"
)

type OpenAIProvider struct{ client openai.Client }

func NewOpenAIProvider(apiKey, baseURL string, timeout time.Duration) OpenAIProvider {
	options := []openaioption.RequestOption{}
	if apiKey != "" {
		options = append(options, openaioption.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		options = append(options, openaioption.WithBaseURL(baseURL))
	}
	if timeout > 0 {
		options = append(options, openaioption.WithRequestTimeout(timeout))
	}
	return OpenAIProvider{client: openai.NewClient(options...)}
}

func (p OpenAIProvider) Name() string { return "openai" }

func (p OpenAIProvider) Complete(ctx context.Context, req Request) (model.CompletionResponse, error) {
	resp, err := p.client.Responses.New(ctx, buildOpenAIRequest(req))
	if err != nil {
		return model.CompletionResponse{}, err
	}
	return completionFromOpenAIResponse(resp), nil
}

func (p OpenAIProvider) Stream(ctx context.Context, req Request, emit func(model.StreamChunk) error) error {
	stream := p.client.Responses.NewStreaming(ctx, buildOpenAIRequest(req))
	for stream.Next() {
		event := stream.Current()
		if delta, ok := openAITextDelta(event); ok {
			if err := emit(model.StreamChunk{Delta: delta}); err != nil {
				return err
			}
		}
		if usage := usageFromOpenAIStreamEvent(event.RawJSON()); usage != nil {
			if err := emit(model.StreamChunk{Usage: usage, Done: true}); err != nil {
				return err
			}
		}
	}
	return stream.Err()
}

func (p OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	pager := p.client.Models.ListAutoPaging(ctx)
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

func buildOpenAIRequest(req Request) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: toOpenAIResponseInput(req.Messages, req.System),
		},
		Model: req.Model,
	}
	if req.System != "" {
		params.Instructions = openai.String(req.System)
	}
	if req.Temperature != nil {
		params.Temperature = openai.Float(*req.Temperature)
	}
	if req.TopP != nil {
		params.TopP = openai.Float(*req.TopP)
	}
	if req.MaxTokens > 0 {
		params.MaxOutputTokens = openai.Int(req.MaxTokens)
	}
	if req.Schema != nil {
		params.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("knotical_response", req.Schema),
		}
	}
	return params
}

func completionFromOpenAIResponse(resp *responses.Response) model.CompletionResponse {
	result := model.CompletionResponse{
		Content: resp.OutputText(),
		Model:   string(resp.Model),
	}
	if resp.Usage.TotalTokens > 0 {
		result.Usage = &model.TokenUsage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		}
	}
	return result
}

func openAITextDelta(event responses.ResponseStreamEventUnion) (string, bool) {
	switch value := event.AsAny().(type) {
	case responses.ResponseTextDeltaEvent:
		return value.Delta, true
	default:
		return "", false
	}
}

func toOpenAIResponseInput(messages []model.Message, system string) []responses.ResponseInputItemUnionParam {
	result := make([]responses.ResponseInputItemUnionParam, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case model.RoleSystem:
			if system != "" {
				continue
			}
			result = append(result, responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRoleSystem))
		case model.RoleAssistant:
			result = append(result, responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRoleAssistant))
		default:
			result = append(result, responses.ResponseInputItemParamOfMessage(msg.Content, responses.EasyInputMessageRoleUser))
		}
	}
	return result
}

func usageFromOpenAIStreamEvent(raw string) *model.TokenUsage {
	if raw == "" {
		return nil
	}
	var payload struct {
		Type     string `json:"type"`
		Response struct {
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	if payload.Type != "response.completed" || payload.Response.Usage.TotalTokens == 0 {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     payload.Response.Usage.InputTokens,
		CompletionTokens: payload.Response.Usage.OutputTokens,
		TotalTokens:      payload.Response.Usage.TotalTokens,
	}
}
