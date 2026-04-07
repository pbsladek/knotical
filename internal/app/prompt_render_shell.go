package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
)

func (s *Service) renderShellResult(ctx context.Context, in renderPromptResultInput) error {
	s.deps.Printer.PrintResponse(in.ResponseText, false)
	report := shell.AnalyzeCommand(in.ResponseText)
	if report.HighRisk {
		s.deps.Printer.Warn("High-risk shell command detected: " + strings.Join(report.Reasons, "; "))
	}
	if in.Request.ExecuteMode != "" {
		return s.executeShellCommand(in.Request, in.ResponseText, report)
	}
	if !in.Request.Interaction {
		return nil
	}
	action, err := s.promptShellAction(in.Request, report)
	if err != nil {
		return err
	}
	switch action {
	case shell.ActionExecuteHost:
		return s.executeShellCommand(withExecuteMode(in.Request, shell.ExecutionModeHost), in.ResponseText, report)
	case shell.ActionExecuteSafe:
		return s.executeShellCommand(withExecuteMode(in.Request, shell.ExecutionModeSafe), in.ResponseText, report)
	case shell.ActionExecuteSandbox:
		return s.executeInteractiveSandboxCommand(ctx, in, report)
	case shell.ActionDescribe:
		return s.describeShellCommand(ctx, in)
	default:
		return nil
	}
}

func (s *Service) promptShellAction(req Request, report shell.RiskReport) (shell.Action, error) {
	if s.deps.PromptAction == nil {
		return "", fmt.Errorf("shell prompt action is not configured")
	}
	return s.deps.PromptAction(shell.PromptOptions{
		HasSandbox: shell.SandboxRuntimeAvailable(req.SandboxRuntime),
		Risk:       report,
	})
}

func (s *Service) executeInteractiveSandboxCommand(ctx context.Context, in renderPromptResultInput, report shell.RiskReport) error {
	command := in.ResponseText
	if !shell.HostCompatibleWithSandbox() {
		regenerated, err := s.generateSandboxCommand(ctx, in)
		if err != nil {
			return err
		}
		command = regenerated
		report = shell.AnalyzeCommand(command)
		s.deps.Printer.Header("sandbox command")
		s.deps.Printer.PrintResponse(command, false)
		if report.HighRisk {
			s.deps.Printer.Warn("High-risk shell command detected: " + strings.Join(report.Reasons, "; "))
		}
	}
	return s.executeShellCommand(withExecuteMode(in.Request, shell.ExecutionModeSandbox), command, report)
}

func (s *Service) generateSandboxCommand(ctx context.Context, in renderPromptResultInput) (string, error) {
	resp, err := s.executePrompt(ctx, in.Provider, provider.Request{
		Model:       in.ModelID,
		Messages:    []model.Message{{Role: model.RoleUser, Content: in.PromptText}},
		System:      shell.SandboxSystemPrompt(),
		Temperature: in.Temperature,
		TopP:        in.TopP,
		MaxTokens:   4096,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func withExecuteMode(req Request, mode shell.ExecutionMode) Request {
	req.ExecuteMode = mode
	return req
}

func (s *Service) describeShellCommand(ctx context.Context, in renderPromptResultInput) error {
	describeReq := provider.Request{
		Model:       in.ModelID,
		Messages:    []model.Message{{Role: model.RoleUser, Content: "Describe what this command does: " + in.ResponseText}},
		System:      "Explain what the shell command does in plain English.",
		Temperature: in.Temperature,
		TopP:        in.TopP,
		MaxTokens:   4096,
		Stream:      in.Config.Stream && !in.Request.NoStream,
	}
	describeResponse, err := s.executePrompt(ctx, in.Provider, describeReq)
	if err != nil {
		return err
	}
	s.deps.Printer.PrintResponse(describeResponse.Content, true)
	return nil
}
