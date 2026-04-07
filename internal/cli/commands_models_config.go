package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
)

func newModelsDefaultCommand(deps modelsCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:  "default <model>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := deps.loadConfig()
			if err != nil {
				return err
			}
			cfg.DefaultModel = args[0]
			return config.Save(cfg)
		},
	}
}

func newModelsInfoCommand(deps modelsCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:  "info <model>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := deps.loadConfig()
			if err != nil {
				return err
			}
			providerCfg := cfg.ProviderSettings()
			providerName, resolvedModel, err := provider.ResolveModel(args[0], "", providerCfg.DefaultProvider)
			if err != nil {
				return err
			}
			message := fmt.Sprintf("Model: %s\nProvider: %s\nCurrent default: %s", args[0], providerName, providerCfg.DefaultModel)
			if resolvedModel != args[0] {
				message += fmt.Sprintf("\nResolved model: %s", resolvedModel)
			}
			output.Println(message)
			return nil
		},
	}
}
