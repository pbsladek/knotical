package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/shell"
)

type rootOptions struct {
	Model          string
	System         string
	Fragments      []string
	Shell          bool
	DescribeShell  bool
	Code           bool
	NoMD           bool
	Chat           string
	Repl           string
	Role           string
	Template       string
	Temperature    float64
	Schema         string
	TopP           float64
	Cache          bool
	Editor         bool
	Interaction    bool
	ContinueLast   bool
	NoStream       bool
	Extract        bool
	Save           string
	Log            bool
	NoLog          bool
	Execute        string
	HostExec       bool
	SafeExec       bool
	SandboxExec    bool
	ForceRisky     bool
	SandboxRuntime string
	DockerRuntime  bool
	PodmanRuntime  bool
	SandboxImage   string
	SandboxNetwork bool
	SandboxWrite   bool
	Prompt         []string
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{Cache: true, Interaction: true, TopP: 1}

	cmd := &cobra.Command{
		Use:   "knotical [PROMPT]",
		Short: "A CLI tool for interacting with LLMs",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Prompt = args
			return runPrompt(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Model, "model", "m", "", "The model to use")
	cmd.Flags().StringVarP(&opts.System, "system", "S", "", "A system prompt to prepend")
	cmd.Flags().StringSliceVar(&opts.Fragments, "fragment", nil, "Append named fragments to the prompt")
	cmd.Flags().BoolVarP(&opts.Shell, "shell", "s", false, "Generate a shell command")
	cmd.Flags().BoolVarP(&opts.DescribeShell, "describe-shell", "d", false, "Describe a shell command")
	cmd.Flags().BoolVarP(&opts.Code, "code", "c", false, "Generate only code output")
	cmd.Flags().BoolVar(&opts.NoMD, "no-md", false, "Disable markdown rendering")
	cmd.Flags().StringVar(&opts.Chat, "chat", "", "Named chat session")
	cmd.Flags().StringVar(&opts.Repl, "repl", "", "Start an interactive REPL session")
	cmd.Flags().StringVar(&opts.Role, "role", "", "Named role")
	cmd.Flags().StringVarP(&opts.Template, "template", "t", "", "Named template")
	cmd.Flags().Float64Var(&opts.Temperature, "temperature", 0, "Sampling temperature")
	cmd.Flags().StringVar(&opts.Schema, "schema", "", "Schema DSL or JSON schema file")
	cmd.Flags().Float64Var(&opts.TopP, "top-p", 1, "Top-p nucleus sampling")
	cmd.Flags().BoolVar(&opts.Cache, "cache", true, "Enable disk cache")
	cmd.Flags().BoolVar(&opts.Editor, "editor", false, "Open $EDITOR to compose the prompt")
	cmd.Flags().BoolVar(&opts.Interaction, "interaction", true, "Allow interactive prompts")
	cmd.Flags().BoolVar(&opts.ContinueLast, "continue", false, "Continue the last chat session")
	cmd.Flags().BoolVar(&opts.NoStream, "no-stream", false, "Disable streaming")
	cmd.Flags().BoolVarP(&opts.Extract, "extract", "x", false, "Extract first fenced code block")
	cmd.Flags().StringVar(&opts.Save, "save", "", "Save current flags as a template")
	cmd.Flags().BoolVar(&opts.Log, "log", false, "Force logging for this invocation")
	cmd.Flags().BoolVarP(&opts.NoLog, "no-log", "n", false, "Disable logging for this invocation")
	cmd.Flags().StringVar(&opts.Execute, "execute", "", "Execute generated shell command using host, safe, or sandbox mode")
	cmd.Flags().BoolVar(&opts.HostExec, "host", false, "Alias for --execute host")
	cmd.Flags().BoolVar(&opts.SafeExec, "safe", false, "Alias for --execute safe")
	cmd.Flags().BoolVar(&opts.SandboxExec, "sandbox", false, "Alias for --execute sandbox")
	cmd.Flags().BoolVar(&opts.ForceRisky, "force-risky-shell", false, "Allow high-risk host shell execution without an extra safety check")
	cmd.Flags().StringVar(&opts.SandboxRuntime, "sandbox-runtime", "", "Sandbox runtime to use for shell execution: docker or podman")
	cmd.Flags().BoolVar(&opts.DockerRuntime, "docker", false, "Alias for --sandbox-runtime docker")
	cmd.Flags().BoolVar(&opts.PodmanRuntime, "podman", false, "Alias for --sandbox-runtime podman")
	cmd.Flags().StringVar(&opts.SandboxImage, "sandbox-image", "", "Container image to use for sandbox shell execution")
	cmd.Flags().StringVar(&opts.SandboxImage, "img", "", "Alias for --sandbox-image")
	cmd.Flags().BoolVar(&opts.SandboxNetwork, "sandbox-network", false, "Allow network access in sandbox shell execution")
	cmd.Flags().BoolVar(&opts.SandboxNetwork, "net", false, "Alias for --sandbox-network")
	cmd.Flags().BoolVar(&opts.SandboxWrite, "sandbox-write", false, "Allow writing to the mounted workspace in sandbox shell execution")
	cmd.Flags().BoolVar(&opts.SandboxWrite, "rw", false, "Alias for --sandbox-write")

	cmd.AddCommand(
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
	if opts.Repl != "" {
		return runRepl(ctx, opts)
	}
	if err := validateRootOptions(opts); err != nil {
		return err
	}
	promptText, err := readPrompt(opts)
	if err != nil {
		return err
	}
	return app.Default(output.NewPrinter(os.Stdout), os.Stdin).RunPrompt(ctx, toAppRequest(opts, promptText))
}

func runRepl(ctx context.Context, opts rootOptions) error {
	if err := normalizeRootOptions(&opts); err != nil {
		return err
	}
	if err := validateRootOptions(opts); err != nil {
		return err
	}
	return app.Default(output.NewPrinter(os.Stdout), os.Stdin).RunRepl(ctx, toAppRequest(opts, ""))
}

func toAppRequest(opts rootOptions, promptText string) app.Request {
	return app.Request{
		PromptText:      promptText,
		Model:           opts.Model,
		System:          opts.System,
		Fragments:       opts.Fragments,
		Shell:           opts.Shell,
		DescribeShell:   opts.DescribeShell,
		Code:            opts.Code,
		NoMD:            opts.NoMD,
		Chat:            opts.Chat,
		Repl:            opts.Repl,
		Role:            opts.Role,
		Template:        opts.Template,
		Temperature:     opts.Temperature,
		Schema:          opts.Schema,
		TopP:            opts.TopP,
		Cache:           opts.Cache,
		Interaction:     opts.Interaction,
		ContinueLast:    opts.ContinueLast,
		NoStream:        opts.NoStream,
		Extract:         opts.Extract,
		Save:            opts.Save,
		Log:             opts.Log,
		NoLog:           opts.NoLog,
		ExecuteMode:     shell.ExecutionMode(opts.Execute),
		ForceRiskyShell: opts.ForceRisky,
		SandboxRuntime:  opts.SandboxRuntime,
		SandboxImage:    opts.SandboxImage,
		SandboxNetwork:  opts.SandboxNetwork,
		SandboxWrite:    opts.SandboxWrite,
	}
}

func validateRootOptions(opts rootOptions) error {
	hasSandboxOptions := opts.SandboxRuntime != "" || opts.SandboxImage != "" || opts.SandboxNetwork || opts.SandboxWrite
	if opts.Log && opts.NoLog {
		return fmt.Errorf("--log and --no-log cannot be used together")
	}
	if opts.Execute != "" {
		if !opts.Shell {
			return fmt.Errorf("--execute requires --shell")
		}
		switch shell.ExecutionMode(opts.Execute) {
		case shell.ExecutionModeHost, shell.ExecutionModeSafe, shell.ExecutionModeSandbox:
		default:
			return fmt.Errorf("--execute must be host, safe, or sandbox")
		}
	}
	if opts.ForceRisky && opts.Execute != string(shell.ExecutionModeHost) {
		return fmt.Errorf("--force-risky-shell requires --execute host")
	}
	if opts.SandboxRuntime != "" && opts.SandboxRuntime != "docker" && opts.SandboxRuntime != "podman" {
		return fmt.Errorf("--sandbox-runtime must be docker or podman")
	}
	if hasSandboxOptions && !opts.Shell {
		return fmt.Errorf("sandbox options require --shell")
	}
	if hasSandboxOptions && opts.Execute != "" && opts.Execute != string(shell.ExecutionModeSandbox) {
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
	runtime := opts.SandboxRuntime
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
	opts.SandboxRuntime = runtime
	return nil
}
