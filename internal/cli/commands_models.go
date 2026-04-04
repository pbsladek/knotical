package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
)

type modelsCommandDeps struct {
	loadConfig    func() (config.Config, error)
	resolveAPIKey func(string) (string, error)
	buildProvider func(string, string, string, time.Duration) (provider.Provider, error)
	providers     []string
	listItem      func(name string, detail string)
	stderr        io.Writer
}

func newModelsCommand() *cobra.Command {
	return newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig:    config.Load,
		resolveAPIKey: resolveAPIKey,
		buildProvider: provider.Build,
		providers:     []string{"openai", "anthropic", "gemini", "ollama"},
		listItem:      output.ListItem,
	})
}

func newModelsCommandWithDeps(deps modelsCommandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "List and inspect models"}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := deps.loadConfig()
				if err != nil {
					return err
				}
				var hardErrors []string
				var missingKeys []string
				listedAny := false
				for _, name := range deps.providers {
					key, err := deps.resolveAPIKey(name)
					if err != nil {
						missingKeys = append(missingKeys, name)
						continue
					}
					prov, err := deps.buildProvider(name, key, cfg.BaseURLForProvider(name), time.Duration(cfg.RequestTimeout)*time.Second)
					if err != nil {
						hardErrors = append(hardErrors, fmt.Sprintf("%s: %v", name, err))
						continue
					}
					models, err := prov.ListModels(cmd.Context())
					if err != nil {
						hardErrors = append(hardErrors, fmt.Sprintf("%s: %v", name, err))
						continue
					}
					for _, model := range models {
						deps.listItem(model, name)
						listedAny = true
					}
				}
				if listedAny {
					errWriter := deps.stderr
					if errWriter == nil {
						errWriter = cmd.ErrOrStderr()
					}
					for _, message := range hardErrors {
						fmt.Fprintln(errWriter, message)
					}
					return nil
				}
				if len(hardErrors) > 0 {
					return fmt.Errorf("models list failed: %s", strings.Join(hardErrors, "; "))
				}
				if len(missingKeys) > 0 {
					return fmt.Errorf("no providers configured; missing API keys for: %s", strings.Join(missingKeys, ", "))
				}
				return fmt.Errorf("no models available")
			},
		},
		&cobra.Command{
			Use:  "default <model>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				cfg.DefaultModel = args[0]
				return config.Save(cfg)
			},
		},
		&cobra.Command{
			Use:  "info <model>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				output.Println(fmt.Sprintf("Model: %s\nProvider: %s\nCurrent default: %s", args[0], provider.DetectProvider(args[0], cfg.DefaultProvider), cfg.DefaultModel))
				return nil
			},
		},
	)
	return cmd
}
