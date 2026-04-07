package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/modelcatalog"
)

type modelListJSON struct {
	Models   []modelcatalog.Entry `json:"models"`
	Warnings []string             `json:"warnings,omitempty"`
}

func newModelsListCommand(deps modelsCommandDeps) *cobra.Command {
	var providerFilter string
	var jsonOutput bool
	var refresh bool
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := deps.loadConfig()
			if err != nil {
				return err
			}
			result, err := modelcatalog.Discover(cmd.Context(), modelcatalog.DiscoveryRequest{
				Config:         cfg,
				Providers:      deps.providers,
				ProviderFilter: providerFilter,
				Refresh:        refresh,
			}, modelcatalog.DiscoveryDeps{
				ResolveAPIKey:    deps.resolveAPIKey,
				BuildProvider:    deps.buildProvider,
				BuildCLIProvider: deps.buildCLIProvider,
				CacheDir:         deps.cacheDir,
				Now:              deps.now,
			})
			if err != nil {
				return err
			}
			return renderModelList(cmd, deps, result, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&providerFilter, "provider", "", "Only list models for a specific provider")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit model listing results as JSON")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Bypass the models discovery cache")
	return cmd
}

func renderModelList(cmd *cobra.Command, deps modelsCommandDeps, result modelcatalog.Result, jsonOutput bool) error {
	if !result.ListedAny() {
		return result.FinalError()
	}
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(modelListJSON{
			Models:   result.Entries,
			Warnings: result.Warnings(),
		})
	}
	for _, entry := range result.Entries {
		detail := fmt.Sprintf("%s (%s)", entry.Provider, entry.Transport)
		if entry.Cached {
			detail += " [cached]"
		}
		deps.listItem(entry.Model, detail)
	}
	writeModelListWarnings(cmd, deps, result)
	return nil
}

func writeModelListWarnings(cmd *cobra.Command, deps modelsCommandDeps, result modelcatalog.Result) {
	errWriter := deps.stderr
	if errWriter == nil {
		errWriter = cmd.ErrOrStderr()
	}
	for _, message := range result.Warnings() {
		fmt.Fprintln(errWriter, message)
	}
}
