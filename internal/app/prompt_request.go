package app

import (
	"context"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
)

type executePromptInput struct {
	Config         config.Config
	Request        Request
	Provider       provider.Provider
	ModelID        string
	SystemPrompt   string
	SchemaValue    map[string]any
	RenderMarkdown bool
	Messages       []model.Message
	Temperature    *float64
	TopP           *float64
}

func (s *Service) executePromptFlow(ctx context.Context, in executePromptInput) (string, model.CompletionResponse, bool, provider.Request, error) {
	responseText, cachedHit := s.lookupCachedPromptResponse(in)
	execReq := buildPromptRequest(in)
	if cachedHit {
		return cachedPromptResult(responseText, execReq)
	}

	completion, err := s.executePrompt(ctx, in.Provider, execReq)
	if err != nil {
		return "", model.CompletionResponse{}, false, provider.Request{}, err
	}
	return completion.Content, completion, false, execReq, nil
}

func (s *Service) lookupCachedPromptResponse(in executePromptInput) (string, bool) {
	if !in.Request.Cache {
		return "", false
	}
	cached, ok, err := s.deps.CacheStore.Get(in.ModelID, in.SystemPrompt, in.Messages, in.SchemaValue, in.Temperature, in.TopP)
	if err != nil || !ok {
		return "", false
	}
	return cached, true
}

func buildPromptRequest(in executePromptInput) provider.Request {
	return provider.Request{
		Model:       in.ModelID,
		Messages:    in.Messages,
		System:      in.SystemPrompt,
		Schema:      in.SchemaValue,
		Temperature: in.Temperature,
		TopP:        in.TopP,
		MaxTokens:   4096,
		Stream:      shouldStreamPrompt(in),
	}
}

func shouldStreamPrompt(in executePromptInput) bool {
	mode := in.Request.ModeOptions
	sampling := in.Request.SamplingOptions
	return in.Config.Stream &&
		!sampling.NoStream &&
		in.SchemaValue == nil &&
		!mode.Shell &&
		!mode.Code &&
		!in.RenderMarkdown
}

func cachedPromptResult(responseText string, execReq provider.Request) (string, model.CompletionResponse, bool, provider.Request, error) {
	execReq.Stream = false
	return responseText, model.CompletionResponse{}, true, execReq, nil
}
