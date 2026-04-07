package app

import (
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/shell"
)

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
