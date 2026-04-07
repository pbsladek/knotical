package cli

import (
	"io"
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
		newModelsDefaultCommand(deps),
		newModelsInfoCommand(deps),
	)
	return cmd
}
