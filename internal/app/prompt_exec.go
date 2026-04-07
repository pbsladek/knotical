package app

import (
	"context"

	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
)

func (s *Service) executePrompt(ctx context.Context, prov provider.Provider, req provider.Request) (model.CompletionResponse, error) {
	var responseText string
	var usage *model.TokenUsage
	if req.Stream {
		err := prov.Stream(ctx, req, func(chunk model.StreamChunk) error {
			if chunk.Delta != "" {
				s.deps.Printer.Print(chunk.Delta)
				responseText += chunk.Delta
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
			return nil
		})
		s.deps.Printer.Println("")
		return model.CompletionResponse{Content: responseText, Model: req.Model, Usage: usage}, err
	}
	return prov.Complete(ctx, req)
}
