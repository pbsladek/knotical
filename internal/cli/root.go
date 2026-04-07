package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
)

type rootOptions struct {
	Request       app.Request
	Editor        bool
	Execute       string
	HostExec      bool
	SafeExec      bool
	SandboxExec   bool
	DockerRuntime bool
	PodmanRuntime bool
	Prompt        []string
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{Request: app.Request{
		RunOptions:      app.RunOptions{Cache: true, Interaction: true},
		SamplingOptions: app.SamplingOptions{TopP: 1},
	}}

	cmd := &cobra.Command{
		Use:   "knotical [PROMPT]",
		Short: "A CLI tool for interacting with LLMs",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Prompt = args
			return runPrompt(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Request.Model, "model", "m", "", "The model to use")
	cmd.Flags().StringVar(&opts.Request.Provider, "provider", "", "Provider to use: openai, anthropic, gemini, or ollama")
	cmd.Flags().StringVarP(&opts.Request.System, "system", "S", "", "A system prompt to prepend")
	cmd.Flags().StringSliceVar(&opts.Request.Fragments, "fragment", nil, "Append named fragments to the prompt")
	cmd.Flags().BoolVarP(&opts.Request.AnalyzeLogs, "analyze-logs", "a", false, "Analyze log input with a log-focused system prompt")
	cmd.Flags().StringVar(&opts.Request.StdinMode, "stdin-mode", "auto", "How to combine piped stdin with the prompt: auto, append, or replace")
	cmd.Flags().StringVar(&opts.Request.StdinLabel, "stdin-label", "input", "Label to use when composing piped stdin into the prompt")
	cmd.Flags().StringSliceVar(&opts.Request.Transforms, "transform", nil, "Apply a named ingest transform to piped stdin")
	cmd.Flags().BoolVar(&opts.Request.NoPipeline, "no-pipeline", false, "Disable any selected or configured log profile/pipeline")
	cmd.Flags().BoolVar(&opts.Request.Clean, "clean", false, "Apply common log sanitizers before reduction")
	cmd.Flags().BoolVar(&opts.Request.Dedupe, "dedupe", false, "Apply exact log-line deduplication")
	cmd.Flags().BoolVar(&opts.Request.Unique, "unique", false, "Collapse repeated log lines into counted unique entries")
	cmd.Flags().BoolVar(&opts.Request.K8s, "k8s", false, "Apply Kubernetes-oriented log normalization")
	cmd.Flags().IntVar(&opts.Request.MaxInputBytes, "max-input-bytes", 0, "Limit piped stdin to this many bytes before sending it")
	cmd.Flags().IntVar(&opts.Request.MaxInputLines, "max-input-lines", 0, "Limit piped stdin to this many lines after reduction")
	cmd.Flags().IntVar(&opts.Request.MaxInputTokens, "max-input-tokens", 0, "Approximate token budget for piped stdin")
	cmd.Flags().StringVar(&opts.Request.InputReduction, "input-reduction", "", "How to handle oversized stdin for token budgeting: off, truncate, fail, or summarize")
	cmd.Flags().IntVar(&opts.Request.SummarizeChunkTokens, "summarize-chunk-tokens", 0, "Approximate token budget per chunk for multi-pass summarization")
	cmd.Flags().StringVar(&opts.Request.SummarizeIntermediateModel, "summarize-intermediate-model", "", "Model to use for intermediate summarization passes")
	cmd.Flags().IntVar(&opts.Request.HeadLines, "head-lines", 0, "Keep only the first N stdin lines")
	cmd.Flags().IntVar(&opts.Request.TailLines, "tail-lines", 0, "Keep only the last N stdin lines")
	cmd.Flags().IntVar(&opts.Request.TailLines, "tail", 0, "Alias for --tail-lines")
	cmd.Flags().IntVar(&opts.Request.SampleLines, "sample-lines", 0, "Keep a deterministic sample of N stdin lines")
	cmd.Flags().BoolVarP(&opts.Request.Shell, "shell", "s", false, "Generate a shell command")
	cmd.Flags().BoolVarP(&opts.Request.DescribeShell, "describe-shell", "d", false, "Describe a shell command")
	cmd.Flags().BoolVarP(&opts.Request.Code, "code", "c", false, "Generate only code output")
	cmd.Flags().BoolVar(&opts.Request.NoMD, "no-md", false, "Disable markdown rendering")
	cmd.Flags().StringVar(&opts.Request.Chat, "chat", "", "Named chat session")
	cmd.Flags().StringVar(&opts.Request.Repl, "repl", "", "Start an interactive REPL session")
	cmd.Flags().StringVarP(&opts.Request.Profile, "profile", "p", "", "Log analysis profile (requires --analyze-logs)")
	cmd.Flags().StringVar(&opts.Request.Role, "role", "", "Named role")
	cmd.Flags().StringVarP(&opts.Request.Template, "template", "t", "", "Named template")
	cmd.Flags().Float64Var(&opts.Request.Temperature, "temperature", 0, "Sampling temperature")
	cmd.Flags().StringVar(&opts.Request.Schema, "schema", "", "Schema DSL or JSON schema file")
	cmd.Flags().Float64Var(&opts.Request.TopP, "top-p", 1, "Top-p nucleus sampling")
	cmd.Flags().BoolVar(&opts.Request.Cache, "cache", true, "Enable disk cache")
	cmd.Flags().BoolVar(&opts.Editor, "editor", false, "Open $EDITOR to compose the prompt")
	cmd.Flags().BoolVar(&opts.Request.Interaction, "interaction", true, "Allow interactive prompts")
	cmd.Flags().BoolVar(&opts.Request.ContinueLast, "continue", false, "Continue the last chat session")
	cmd.Flags().BoolVar(&opts.Request.NoStream, "no-stream", false, "Disable streaming")
	cmd.Flags().BoolVarP(&opts.Request.Extract, "extract", "x", false, "Extract first fenced code block")
	cmd.Flags().StringVar(&opts.Request.Save, "save", "", "Save current flags as a template")
	cmd.Flags().BoolVar(&opts.Request.Log, "log", false, "Force logging for this invocation")
	cmd.Flags().BoolVarP(&opts.Request.NoLog, "no-log", "n", false, "Disable logging for this invocation")
	cmd.Flags().StringVar(&opts.Execute, "execute", "", "Execute generated shell command using host, safe, or sandbox mode")
	cmd.Flags().BoolVar(&opts.HostExec, "host", false, "Alias for --execute host")
	cmd.Flags().BoolVar(&opts.SafeExec, "safe", false, "Alias for --execute safe")
	cmd.Flags().BoolVar(&opts.SandboxExec, "sandbox", false, "Alias for --execute sandbox")
	cmd.Flags().BoolVar(&opts.Request.ForceRiskyShell, "force-risky-shell", false, "Allow high-risk host shell execution without an extra safety check")
	cmd.Flags().StringVar(&opts.Request.SandboxRuntime, "sandbox-runtime", "", "Sandbox runtime to use for shell execution: docker or podman")
	cmd.Flags().BoolVar(&opts.DockerRuntime, "docker", false, "Alias for --sandbox-runtime docker")
	cmd.Flags().BoolVar(&opts.PodmanRuntime, "podman", false, "Alias for --sandbox-runtime podman")
	cmd.Flags().StringVar(&opts.Request.SandboxImage, "sandbox-image", "", "Container image to use for sandbox shell execution")
	cmd.Flags().StringVar(&opts.Request.SandboxImage, "img", "", "Alias for --sandbox-image")
	cmd.Flags().BoolVar(&opts.Request.SandboxNetwork, "sandbox-network", false, "Allow network access in sandbox shell execution")
	cmd.Flags().BoolVar(&opts.Request.SandboxNetwork, "net", false, "Alias for --sandbox-network")
	cmd.Flags().BoolVar(&opts.Request.SandboxWrite, "sandbox-write", false, "Allow writing to the mounted workspace in sandbox shell execution")
	cmd.Flags().BoolVar(&opts.Request.SandboxWrite, "rw", false, "Alias for --sandbox-write")

	cmd.AddCommand(
		newConfigCommand(),
		newKeysCommand(),
		newLogsCommand(),
		newModelsCommand(),
		newChatsCommand(),
		newRolesCommand(),
		newTemplatesCommand(),
		newFragmentsCommand(),
		newAliasesCommand(),
		newInstallIntegrationCommand(),
	)

	return cmd
}

func runPrompt(ctx context.Context, opts rootOptions) error {
	if err := normalizeRootOptions(&opts); err != nil {
		return err
	}
	if opts.Request.Repl != "" {
		return runRepl(ctx, opts)
	}
	if err := validateRootOptions(opts); err != nil {
		return err
	}
	prompt, err := readPromptSource(opts)
	if err != nil {
		return err
	}
	return app.Default(output.NewPrinter(os.Stdout), os.Stdin).RunPrompt(ctx, toAppRequest(opts, prompt))
}

func runRepl(ctx context.Context, opts rootOptions) error {
	if err := normalizeRootOptions(&opts); err != nil {
		return err
	}
	if err := validateRootOptions(opts); err != nil {
		return err
	}
	return app.Default(output.NewPrinter(os.Stdout), os.Stdin).RunRepl(ctx, toAppRequest(opts, promptSource{}))
}

func toAppRequest(opts rootOptions, prompt promptSource) app.Request {
	req := opts.Request
	req.PromptText = prompt.instructionText
	req.StdinText = prompt.stdinText
	req.Transforms = append([]string(nil), req.Transforms...)
	req.Fragments = append([]string(nil), req.Fragments...)
	req.ExecuteMode = shell.ExecutionMode(opts.Execute)
	return req
}

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
