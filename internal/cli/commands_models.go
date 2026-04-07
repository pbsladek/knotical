package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
)

type modelsCommandDeps struct {
	loadConfig       func() (config.Config, error)
	resolveAPIKey    func(string) (string, error)
	buildProvider    func(string, string, string, time.Duration) (provider.Provider, error)
	buildCLIProvider func(string, provider.CLIConfig) (provider.Provider, error)
	cacheDir         func() string
	now              func() time.Time
	providers        []string
	listItem         func(name string, detail string)
	stderr           io.Writer
}

func newModelsCommand() *cobra.Command {
	return newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig:       config.Load,
		resolveAPIKey:    resolveAPIKey,
		buildProvider:    provider.Build,
		buildCLIProvider: provider.BuildCLI,
		cacheDir:         config.CacheDir,
		now:              func() time.Time { return time.Now().UTC() },
		providers:        []string{"openai", "anthropic", "gemini", "ollama"},
		listItem:         output.ListItem,
	})
}

func newModelsCommandWithDeps(deps modelsCommandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "List and inspect models"}
	cmd.AddCommand(
		newModelsListCommand(deps),
		newModelsDefaultCommand(),
		newModelsInfoCommand(),
	)
	return cmd
}

type modelListOutcome struct {
	listedAny   bool
	entries     []modelListEntry
	hardErrors  []string
	missingKeys []string
	unsupported []string
}

func newModelsListCommand(deps modelsCommandDeps) *cobra.Command {
	var providerFilter string
	var jsonOutput bool
	var refresh bool
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if providerFilter != "" && !provider.IsKnownProvider(providerFilter) {
				return fmt.Errorf("--provider must be openai, anthropic, gemini, or ollama")
			}
			cfg, err := deps.loadConfig()
			if err != nil {
				return err
			}
			outcome := modelListOutcome{}
			for _, name := range filteredModelProviders(deps.providers, providerFilter) {
				if err := listProviderModels(cmd, deps, cfg, name, refresh, &outcome); err != nil {
					return err
				}
			}
			return finalizeModelList(cmd, deps, outcome, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&providerFilter, "provider", "", "Only list models for a specific provider")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit model listing results as JSON")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Bypass the models discovery cache")
	return cmd
}

type modelListEntry struct {
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	Transport string `json:"transport"`
	Cached    bool   `json:"cached,omitempty"`
}

type modelListJSON struct {
	Models   []modelListEntry `json:"models"`
	Warnings []string         `json:"warnings,omitempty"`
}

type modelListCacheEntry struct {
	Transport string    `json:"transport"`
	BaseURL   string    `json:"base_url"`
	FetchedAt time.Time `json:"fetched_at"`
	Models    []string  `json:"models"`
}

const modelListCacheTTL = time.Hour

func filteredModelProviders(providers []string, providerFilter string) []string {
	if providerFilter == "" {
		return providers
	}
	for _, name := range providers {
		if name == providerFilter {
			return []string{name}
		}
	}
	return []string{providerFilter}
}

func listProviderModels(cmd *cobra.Command, deps modelsCommandDeps, cfg config.Config, name string, refresh bool, outcome *modelListOutcome) error {
	runtimeCfg := cfg.ProviderRuntime(name)
	if !runtimeCfg.Capabilities.ModelListing {
		outcome.unsupported = append(outcome.unsupported, fmt.Sprintf("%s (%s)", name, runtimeCfg.Transport))
		return nil
	}
	if !refresh {
		if models, ok := loadModelsListCache(deps, name, runtimeCfg); ok {
			appendListedModels(outcome, name, runtimeCfg.Transport, models, true)
			return nil
		}
	}
	prov, err := buildModelListProvider(deps, cfg, name, runtimeCfg)
	if err != nil {
		if errors.Is(err, errMissingProviderKey) {
			outcome.missingKeys = append(outcome.missingKeys, name)
			return nil
		}
		outcome.hardErrors = append(outcome.hardErrors, fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	models, err := prov.ListModels(cmd.Context())
	if err != nil {
		if errors.Is(err, provider.ErrModelListingUnsupported) {
			outcome.unsupported = append(outcome.unsupported, fmt.Sprintf("%s (%s)", name, runtimeCfg.Transport))
			return nil
		}
		outcome.hardErrors = append(outcome.hardErrors, fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	saveModelsListCache(deps, name, runtimeCfg, models)
	appendListedModels(outcome, name, runtimeCfg.Transport, models, false)
	return nil
}

var errMissingProviderKey = errors.New("missing provider key")

func appendListedModels(outcome *modelListOutcome, providerName string, transport string, models []string, cached bool) {
	for _, name := range models {
		outcome.entries = append(outcome.entries, modelListEntry{
			Model:     name,
			Provider:  providerName,
			Transport: transport,
			Cached:    cached,
		})
		outcome.listedAny = true
	}
}

func loadModelsListCache(deps modelsCommandDeps, providerName string, runtimeCfg config.ProviderRuntime) ([]string, bool) {
	cacheDir := ""
	if deps.cacheDir != nil {
		cacheDir = deps.cacheDir()
	}
	if cacheDir == "" {
		return nil, false
	}
	payload, err := os.ReadFile(filepath.Join(cacheDir, "models-"+providerName+".json"))
	if err != nil {
		return nil, false
	}
	var entry modelListCacheEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return nil, false
	}
	now := time.Now().UTC()
	if deps.now != nil {
		now = deps.now()
	}
	if entry.Transport != runtimeCfg.Transport || entry.BaseURL != runtimeCfg.BaseURL || now.Sub(entry.FetchedAt) > modelListCacheTTL {
		return nil, false
	}
	return append([]string(nil), entry.Models...), true
}

func saveModelsListCache(deps modelsCommandDeps, providerName string, runtimeCfg config.ProviderRuntime, models []string) {
	cacheDir := ""
	if deps.cacheDir != nil {
		cacheDir = deps.cacheDir()
	}
	if cacheDir == "" {
		return
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return
	}
	now := time.Now().UTC()
	if deps.now != nil {
		now = deps.now()
	}
	payload, err := json.MarshalIndent(modelListCacheEntry{
		Transport: runtimeCfg.Transport,
		BaseURL:   runtimeCfg.BaseURL,
		FetchedAt: now,
		Models:    append([]string(nil), models...),
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(cacheDir, "models-"+providerName+".json"), payload, 0o600)
}

func buildModelListProvider(deps modelsCommandDeps, cfg config.Config, name string, runtimeCfg config.ProviderRuntime) (provider.Provider, error) {
	if runtimeCfg.Transport == "cli" {
		return deps.buildCLIProvider(name, provider.CLIConfig(runtimeCfg.CLI))
	}
	key, err := deps.resolveAPIKey(name)
	if err != nil {
		return nil, errMissingProviderKey
	}
	return deps.buildProvider(name, key, runtimeCfg.BaseURL, cfg.ProviderSettings().RequestTimeout)
}

func finalizeModelList(cmd *cobra.Command, deps modelsCommandDeps, outcome modelListOutcome, jsonOutput bool) error {
	if outcome.listedAny {
		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(modelListJSON{
				Models:   outcome.entries,
				Warnings: collectModelListWarnings(outcome),
			})
		}
		for _, entry := range outcome.entries {
			detail := fmt.Sprintf("%s (%s)", entry.Provider, entry.Transport)
			if entry.Cached {
				detail += " [cached]"
			}
			deps.listItem(entry.Model, detail)
		}
		writeModelListWarnings(cmd, deps, outcome)
		return nil
	}
	if len(outcome.hardErrors) > 0 {
		return fmt.Errorf("models list failed: %s", strings.Join(outcome.hardErrors, "; "))
	}
	if len(outcome.unsupported) > 0 && len(outcome.missingKeys) == 0 {
		return fmt.Errorf("models list is not supported for configured CLI transports: %s", strings.Join(outcome.unsupported, ", "))
	}
	if len(outcome.missingKeys) > 0 {
		return fmt.Errorf("no providers configured; missing API keys for: %s", strings.Join(outcome.missingKeys, ", "))
	}
	return fmt.Errorf("no models available")
}

func collectModelListWarnings(outcome modelListOutcome) []string {
	warnings := make([]string, 0, len(outcome.hardErrors)+len(outcome.unsupported))
	warnings = append(warnings, outcome.hardErrors...)
	for _, name := range outcome.unsupported {
		warnings = append(warnings, fmt.Sprintf("%s: model listing is not supported for this transport", name))
	}
	return warnings
}

func writeModelListWarnings(cmd *cobra.Command, deps modelsCommandDeps, outcome modelListOutcome) {
	errWriter := deps.stderr
	if errWriter == nil {
		errWriter = cmd.ErrOrStderr()
	}
	for _, message := range collectModelListWarnings(outcome) {
		fmt.Fprintln(errWriter, message)
	}
}

func newModelsDefaultCommand() *cobra.Command {
	return &cobra.Command{
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
	}
}

func newModelsInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:  "info <model>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
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
