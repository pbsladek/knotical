package app

import (
	"context"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/provider"
)

type renderPromptResultInput struct {
	Request        Request
	Config         config.Config
	Provider       provider.Provider
	ModelID        string
	PromptText     string
	ResponseText   string
	RenderMarkdown bool
	Temperature    *float64
	TopP           *float64
	ExecRequest    provider.Request
}

func (s *Service) renderPromptResult(ctx context.Context, in renderPromptResultInput) error {
	switch classifyPromptRenderMode(in) {
	case promptRenderModeShell:
		return s.renderShellResult(ctx, in)
	case promptRenderModePlain:
		s.deps.Printer.PrintResponse(in.ResponseText, false)
	case promptRenderModeMarkdown:
		s.deps.Printer.PrintResponse(in.ResponseText, in.RenderMarkdown)
	}
	return nil
}

type promptRenderMode int

const (
	promptRenderModeSilent promptRenderMode = iota
	promptRenderModeShell
	promptRenderModePlain
	promptRenderModeMarkdown
)

func classifyPromptRenderMode(in renderPromptResultInput) promptRenderMode {
	mode := in.Request.ModeOptions
	switch {
	case mode.Shell:
		return promptRenderModeShell
	case mode.Code || in.Request.Schema != "" || mode.DescribeShell:
		return promptRenderModePlain
	case !in.ExecRequest.Stream:
		return promptRenderModeMarkdown
	default:
		return promptRenderModeSilent
	}
}
