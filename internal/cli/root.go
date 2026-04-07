package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/output"
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
	opts := newRootOptions()
	cmd := &cobra.Command{
		Use:   "knotical [PROMPT]",
		Short: "A CLI tool for interacting with LLMs",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Prompt = args
			return runPrompt(cmd.Context(), *opts)
		},
	}

	registerRootFlags(cmd, opts)
	addRootSubcommands(cmd)
	return cmd
}

func newRootOptions() *rootOptions {
	return &rootOptions{Request: app.Request{
		RunOptions:      app.RunOptions{Cache: true, Interaction: true},
		SamplingOptions: app.SamplingOptions{TopP: 1},
	}}
}

func addRootSubcommands(cmd *cobra.Command) {
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
}

func defaultApp() *app.Service {
	return app.Default(output.NewPrinter(os.Stdout), os.Stdin)
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
	return defaultApp().RunPrompt(ctx, toAppRequest(opts, prompt))
}

func runRepl(ctx context.Context, opts rootOptions) error {
	if err := normalizeRootOptions(&opts); err != nil {
		return err
	}
	if err := validateRootOptions(opts); err != nil {
		return err
	}
	return defaultApp().RunRepl(ctx, toAppRequest(opts, promptSource{}))
}
