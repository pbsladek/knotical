package provider

import (
	"context"
	"errors"

	"github.com/pbsladek/knotical/internal/model"
)

type Request struct {
	Model       string
	Messages    []model.Message
	System      string
	Schema      map[string]any
	Temperature *float64
	TopP        *float64
	MaxTokens   int64
	Stream      bool
}

type Provider interface {
	Name() string
	Complete(context.Context, Request) (model.CompletionResponse, error)
	Stream(context.Context, Request, func(model.StreamChunk) error) error
	ListModels(context.Context) ([]string, error)
}

var ErrModelListingUnsupported = errors.New("model listing is not supported")
