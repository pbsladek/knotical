package provider

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/pbsladek/knotical/internal/model"
)

type GeminiProvider struct{ client *genai.Client }

func NewGeminiProvider(apiKey string, baseURL string, timeout time.Duration) (GeminiProvider, error) {
	return newGeminiProvider(apiKey, baseURL, nil, timeout)
}

func newGeminiProvider(apiKey string, baseURL string, httpClient *http.Client, timeout time.Duration) (GeminiProvider, error) {
	ctx := context.Background()
	config := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if baseURL != "" {
		config.HTTPOptions.BaseURL = baseURL
	}
	if timeout > 0 && httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	if httpClient != nil {
		config.HTTPClient = httpClient
	}
	client, err := genai.NewClient(ctx, config)
	if err != nil {
		return GeminiProvider{}, err
	}
	return GeminiProvider{client: client}, nil
}

func (p GeminiProvider) Name() string { return "gemini" }

func (p GeminiProvider) Complete(ctx context.Context, req Request) (model.CompletionResponse, error) {
	if p.client == nil {
		return model.CompletionResponse{}, fmt.Errorf("gemini client initialization failed")
	}
	resp, err := p.client.Models.GenerateContent(ctx, req.Model, toGeminiContents(req.Messages), toGeminiConfig(req))
	if err != nil {
		return model.CompletionResponse{}, err
	}
	result := model.CompletionResponse{Content: textFromGenAI(resp), Model: req.Model}
	if usage := usageFromGenAI(resp); usage != nil {
		result.Usage = usage
	}
	return result, nil
}

func (p GeminiProvider) Stream(ctx context.Context, req Request, emit func(model.StreamChunk) error) error {
	if p.client == nil {
		return fmt.Errorf("gemini client initialization failed")
	}
	var usage *model.TokenUsage
	for resp, err := range p.client.Models.GenerateContentStream(ctx, req.Model, toGeminiContents(req.Messages), toGeminiConfig(req)) {
		if err != nil {
			return err
		}
		if text := textFromGenAI(resp); text != "" {
			if err := emit(model.StreamChunk{Delta: text}); err != nil {
				return err
			}
		}
		if currentUsage := usageFromGenAI(resp); currentUsage != nil {
			usage = currentUsage
		}
	}
	if usage != nil {
		if err := emit(model.StreamChunk{Usage: usage, Done: true}); err != nil {
			return err
		}
	}
	return nil
}

func (p GeminiProvider) ListModels(ctx context.Context) ([]string, error) {
	if p.client == nil {
		return nil, fmt.Errorf("gemini client initialization failed")
	}
	models := []string{}
	for item, err := range p.client.Models.All(ctx) {
		if err != nil {
			return nil, err
		}
		if item == nil || item.Name == "" {
			continue
		}
		models = append(models, strings.TrimPrefix(item.Name, "models/"))
	}
	slices.Sort(models)
	return models, nil
}

func toGeminiContents(messages []model.Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case model.RoleSystem:
			continue
		case model.RoleAssistant:
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleModel))
		default:
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleUser))
		}
	}
	return contents
}

func toGeminiConfig(req Request) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}
	if req.System != "" {
		config.SystemInstruction = genai.NewContentFromText(req.System, "system")
	}
	if req.Temperature != nil {
		value := float32(*req.Temperature)
		config.Temperature = &value
	}
	if req.TopP != nil {
		value := float32(*req.TopP)
		config.TopP = &value
	}
	if req.MaxTokens > 0 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}
	if req.Schema != nil {
		config.ResponseMIMEType = "application/json"
		config.ResponseJsonSchema = req.Schema
	}
	if config.SystemInstruction == nil &&
		config.Temperature == nil &&
		config.TopP == nil &&
		config.MaxOutputTokens == 0 &&
		config.ResponseMIMEType == "" &&
		config.ResponseJsonSchema == nil {
		return nil
	}
	return config
}

func textFromGenAI(resp *genai.GenerateContentResponse) string {
	if resp == nil {
		return ""
	}
	var builder strings.Builder
	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func usageFromGenAI(resp *genai.GenerateContentResponse) *model.TokenUsage {
	if resp == nil || resp.UsageMetadata == nil {
		return nil
	}
	usage := resp.UsageMetadata
	if usage.PromptTokenCount == 0 && usage.CandidatesTokenCount == 0 && usage.TotalTokenCount == 0 {
		return nil
	}
	return &model.TokenUsage{
		PromptTokens:     int(usage.PromptTokenCount),
		CompletionTokens: int(usage.CandidatesTokenCount),
		TotalTokens:      int(usage.TotalTokenCount),
	}
}
