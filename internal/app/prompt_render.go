package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
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
	mode := in.Request.ModeOptions
	switch {
	case mode.Shell:
		return s.renderShellResult(ctx, in)
	case mode.Code || in.Request.Schema != "" || mode.DescribeShell:
		s.deps.Printer.PrintResponse(in.ResponseText, false)
	case !in.ExecRequest.Stream:
		s.deps.Printer.PrintResponse(in.ResponseText, in.RenderMarkdown)
	}
	return nil
}

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
	if s.deps.PromptAction == nil {
		return fmt.Errorf("shell prompt action is not configured")
	}
	action, err := s.deps.PromptAction(shell.PromptOptions{
		HasSandbox: shell.SandboxRuntimeAvailable(in.Request.SandboxRuntime),
		Risk:       report,
	})
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

func (s *Service) executeShellCommand(req Request, command string, report shell.RiskReport) error {
	execReq := buildShellExecutionRequest(req, command)
	allowed, err := s.validateShellExecution(req, execReq, report)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	if s.deps.ExecuteShell == nil {
		return fmt.Errorf("shell execution is not configured")
	}
	return s.deps.ExecuteShell(execReq)
}

func buildShellExecutionRequest(req Request, command string) shell.ExecutionRequest {
	return shell.ExecutionRequest{
		Command: command,
		Mode:    req.ExecuteMode,
		Sandbox: shell.SandboxConfig{
			Runtime: req.SandboxRuntime,
			Image:   req.SandboxImage,
			Network: req.SandboxNetwork,
			Write:   req.SandboxWrite,
		},
	}
}

func (s *Service) validateShellExecution(req Request, execReq shell.ExecutionRequest, report shell.RiskReport) (bool, error) {
	allowed, err := s.confirmRiskyHostExecution(req, execReq, report)
	if err != nil || !allowed {
		return allowed, err
	}
	if execReq.Mode == shell.ExecutionModeSafe && report.HighRisk {
		return false, fmt.Errorf("safe shell execution refuses high-risk commands: %s", strings.Join(report.Reasons, "; "))
	}
	return true, nil
}

func (s *Service) confirmRiskyHostExecution(req Request, execReq shell.ExecutionRequest, report shell.RiskReport) (bool, error) {
	if execReq.Mode != shell.ExecutionModeHost || !report.HighRisk || req.ForceRiskyShell {
		return true, nil
	}
	if !req.Interaction {
		return false, fmt.Errorf("refusing high-risk host shell execution without --force-risky-shell")
	}
	if s.deps.ConfirmShell == nil {
		return false, fmt.Errorf("shell confirmation is not configured")
	}
	ok, err := s.deps.ConfirmShell(execReq.Mode, report)
	if err != nil {
		return false, err
	}
	return ok, nil
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
