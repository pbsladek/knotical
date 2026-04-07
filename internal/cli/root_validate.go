package cli

import (
	"fmt"

	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
)

func validateRootOptions(opts rootOptions) error {
	req := opts.Request
	if err := validateLoggingOptions(req); err != nil {
		return err
	}
	if err := validateInputOptions(req, hasExplicitPipelineOptions(req)); err != nil {
		return err
	}
	if err := validateModeOptions(req); err != nil {
		return err
	}
	return validateExecutionOptions(req, opts.Execute, hasSandboxOptions(req))
}

func hasSandboxOptions(req app.Request) bool {
	return req.SandboxRuntime != "" || req.SandboxImage != "" || req.SandboxNetwork || req.SandboxWrite
}

func hasExplicitPipelineOptions(req app.Request) bool {
	return req.Profile != "" || len(req.Transforms) > 0 || req.Clean || req.Dedupe || req.Unique || req.K8s
}

func validateLoggingOptions(req app.Request) error {
	if req.Log && req.NoLog {
		return fmt.Errorf("--log and --no-log cannot be used together")
	}
	return nil
}

func validateInputOptions(req app.Request, hasExplicitPipelineOptions bool) error {
	switch req.StdinMode {
	case "", "auto", "append", "replace":
	default:
		return fmt.Errorf("--stdin-mode must be auto, append, or replace")
	}
	switch req.InputReduction {
	case "", "off", "truncate", "fail", "summarize":
	default:
		return fmt.Errorf("--input-reduction must be off, truncate, fail, or summarize")
	}
	if req.NoPipeline && hasExplicitPipelineOptions {
		return fmt.Errorf("--no-pipeline cannot be combined with --profile, --transform, --clean, --dedupe, --unique, or --k8s")
	}
	return validateNonNegativeInputs(req)
}

func validateNonNegativeInputs(req app.Request) error {
	for _, value := range []struct {
		name string
		v    int
	}{
		{name: "--max-input-bytes", v: req.MaxInputBytes},
		{name: "--max-input-lines", v: req.MaxInputLines},
		{name: "--max-input-tokens", v: req.MaxInputTokens},
		{name: "--summarize-chunk-tokens", v: req.SummarizeChunkTokens},
		{name: "--head-lines", v: req.HeadLines},
		{name: "--tail-lines", v: req.TailLines},
		{name: "--sample-lines", v: req.SampleLines},
	} {
		if value.v < 0 {
			return fmt.Errorf("%s must be >= 0", value.name)
		}
	}
	return nil
}

func validateModeOptions(req app.Request) error {
	modeCount := 0
	for _, enabled := range []bool{req.AnalyzeLogs, req.Shell, req.Code, req.DescribeShell} {
		if enabled {
			modeCount++
		}
	}
	if modeCount > 1 {
		return fmt.Errorf("--analyze-logs cannot be combined with --shell, --code, or --describe-shell")
	}
	if req.Profile != "" && !req.AnalyzeLogs {
		return fmt.Errorf("--profile requires --analyze-logs")
	}
	if req.Provider != "" && !provider.IsKnownProvider(req.Provider) {
		return fmt.Errorf("--provider must be openai, anthropic, gemini, or ollama")
	}
	return nil
}

func validateExecutionOptions(req app.Request, execute string, hasSandboxOptions bool) error {
	if err := validateExecuteMode(req, execute); err != nil {
		return err
	}
	if req.ForceRiskyShell && execute != string(shell.ExecutionModeHost) {
		return fmt.Errorf("--force-risky-shell requires --execute host")
	}
	return validateSandboxOptions(req, execute, hasSandboxOptions)
}

func validateExecuteMode(req app.Request, execute string) error {
	if execute == "" {
		return nil
	}
	if !req.Shell {
		return fmt.Errorf("--execute requires --shell")
	}
	switch shell.ExecutionMode(execute) {
	case shell.ExecutionModeHost, shell.ExecutionModeSafe, shell.ExecutionModeSandbox:
		return nil
	default:
		return fmt.Errorf("--execute must be host, safe, or sandbox")
	}
}

func validateSandboxOptions(req app.Request, execute string, hasSandboxOptions bool) error {
	if req.SandboxRuntime != "" && req.SandboxRuntime != "docker" && req.SandboxRuntime != "podman" {
		return fmt.Errorf("--sandbox-runtime must be docker or podman")
	}
	if hasSandboxOptions && !req.Shell {
		return fmt.Errorf("sandbox options require --shell")
	}
	if hasSandboxOptions && execute != "" && execute != string(shell.ExecutionModeSandbox) {
		return fmt.Errorf("sandbox options require --execute sandbox when execution mode is set explicitly")
	}
	return nil
}

func normalizeRootOptions(opts *rootOptions) error {
	if err := normalizeExecuteAliases(opts); err != nil {
		return err
	}
	return normalizeRuntimeAliases(opts)
}

func normalizeExecuteAliases(opts *rootOptions) error {
	mode := opts.Execute
	for _, candidate := range []struct {
		enabled bool
		value   string
		flag    string
	}{
		{enabled: opts.HostExec, value: string(shell.ExecutionModeHost), flag: "--host"},
		{enabled: opts.SafeExec, value: string(shell.ExecutionModeSafe), flag: "--safe"},
		{enabled: opts.SandboxExec, value: string(shell.ExecutionModeSandbox), flag: "--sandbox"},
	} {
		if !candidate.enabled {
			continue
		}
		if mode != "" && mode != candidate.value {
			return fmt.Errorf("%s conflicts with --execute %s", candidate.flag, mode)
		}
		mode = candidate.value
	}
	opts.Execute = mode
	return nil
}

func normalizeRuntimeAliases(opts *rootOptions) error {
	runtime := opts.Request.SandboxRuntime
	for _, candidate := range []struct {
		enabled bool
		value   string
		flag    string
	}{
		{enabled: opts.DockerRuntime, value: "docker", flag: "--docker"},
		{enabled: opts.PodmanRuntime, value: "podman", flag: "--podman"},
	} {
		if !candidate.enabled {
			continue
		}
		if runtime != "" && runtime != candidate.value {
			return fmt.Errorf("%s conflicts with --sandbox-runtime %s", candidate.flag, runtime)
		}
		runtime = candidate.value
	}
	opts.Request.SandboxRuntime = runtime
	return nil
}
