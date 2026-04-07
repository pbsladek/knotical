package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
)

type modelsTestProvider struct {
	models []string
	err    error
}

func (p modelsTestProvider) Name() string { return "test" }

func (p modelsTestProvider) Complete(context.Context, provider.Request) (model.CompletionResponse, error) {
	return model.CompletionResponse{}, nil
}

func (p modelsTestProvider) Stream(context.Context, provider.Request, func(model.StreamChunk) error) error {
	return nil
}

func (p modelsTestProvider) ListModels(context.Context) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.models, nil
}

func TestModelsListReturnsErrorWhenEveryProviderFails(t *testing.T) {
	cmd := newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig:    func() (config.Config, error) { return config.Default(), nil },
		resolveAPIKey: func(string) (string, error) { return "key", nil },
		buildProvider: func(string, string, string, time.Duration) (provider.Provider, error) {
			return modelsTestProvider{err: errors.New("auth failed")}, nil
		},
		now:       func() time.Time { return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC) },
		providers: []string{"openai"},
		listItem:  func(string, string) {},
	})
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected models list error")
	}
	if !strings.Contains(err.Error(), "auth failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelsListWarnsButSucceedsWhenSomeProvidersWork(t *testing.T) {
	var listed []string
	var stderr strings.Builder
	cmd := newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig:    func() (config.Config, error) { return config.Default(), nil },
		resolveAPIKey: func(string) (string, error) { return "key", nil },
		buildProvider: func(name string, _ string, _ string, _ time.Duration) (provider.Provider, error) {
			if name == "anthropic" {
				return modelsTestProvider{err: errors.New("temporary failure")}, nil
			}
			return modelsTestProvider{models: []string{"gpt-4o-mini"}}, nil
		},
		providers: []string{"openai", "anthropic"},
		listItem: func(name string, detail string) {
			listed = append(listed, fmt.Sprintf("%s:%s", name, detail))
		},
		now:    func() time.Time { return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC) },
		stderr: &stderr,
	})
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models list failed: %v", err)
	}
	if len(listed) != 1 || listed[0] != "gpt-4o-mini:openai (api)" {
		t.Fatalf("expected listed model output, got %+v", listed)
	}
	if !strings.Contains(stderr.String(), "temporary failure") {
		t.Fatalf("expected warning output, got %q", stderr.String())
	}
}

func TestModelsListUsesProviderScopedBaseURL(t *testing.T) {
	var gotBaseURL string
	cmd := newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.OpenAIBaseURL = "http://openai.local"
			cfg.APIBaseURL = "http://legacy-ollama.local/v1"
			return cfg, nil
		},
		resolveAPIKey: func(string) (string, error) { return "key", nil },
		buildProvider: func(name, _ string, baseURL string, _ time.Duration) (provider.Provider, error) {
			if name == "openai" {
				gotBaseURL = baseURL
			}
			return modelsTestProvider{models: []string{"gpt-4o-mini"}}, nil
		},
		now:       func() time.Time { return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC) },
		providers: []string{"openai"},
		listItem:  func(string, string) {},
	})
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models list failed: %v", err)
	}
	if gotBaseURL != "http://openai.local" {
		t.Fatalf("expected provider-scoped base URL, got %q", gotBaseURL)
	}
}

func TestModelsListUsesCLITransportWithoutAPIKey(t *testing.T) {
	var builtCLI bool
	cmd := newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.AnthropicTransport = "cli"
			return cfg, nil
		},
		resolveAPIKey: func(string) (string, error) {
			return "", errors.New("should not request api key")
		},
		buildProvider: func(string, string, string, time.Duration) (provider.Provider, error) {
			return nil, errors.New("should not build api provider")
		},
		buildCLIProvider: func(name string, cfg provider.CLIConfig) (provider.Provider, error) {
			builtCLI = true
			return modelsTestProvider{}, nil
		},
		now:       func() time.Time { return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC) },
		providers: []string{"anthropic"},
		listItem:  func(string, string) {},
	})
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected models list error")
	}
	if builtCLI {
		t.Fatal("did not expect cli provider build for unsupported model listing transport")
	}
	if !strings.Contains(err.Error(), "not supported for configured CLI transports") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelsListFiltersByProvider(t *testing.T) {
	var built []string
	var listed []string
	cmd := newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig:    func() (config.Config, error) { return config.Default(), nil },
		resolveAPIKey: func(string) (string, error) { return "key", nil },
		buildProvider: func(name string, _ string, _ string, _ time.Duration) (provider.Provider, error) {
			built = append(built, name)
			return modelsTestProvider{models: []string{"gpt-4o-mini"}}, nil
		},
		now:       func() time.Time { return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC) },
		providers: []string{"openai", "anthropic"},
		listItem: func(name string, detail string) {
			listed = append(listed, fmt.Sprintf("%s:%s", name, detail))
		},
	})
	cmd.SetArgs([]string{"list", "--provider", "openai"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models list failed: %v", err)
	}
	if len(built) != 1 || built[0] != "openai" {
		t.Fatalf("expected only openai provider build, got %+v", built)
	}
	if len(listed) != 1 || listed[0] != "gpt-4o-mini:openai (api)" {
		t.Fatalf("unexpected listed output: %+v", listed)
	}
}

func TestModelsListOutputsJSON(t *testing.T) {
	var stdout strings.Builder
	cmd := newModelsCommandWithDeps(modelsCommandDeps{
		loadConfig:    func() (config.Config, error) { return config.Default(), nil },
		resolveAPIKey: func(string) (string, error) { return "key", nil },
		buildProvider: func(string, string, string, time.Duration) (provider.Provider, error) {
			return modelsTestProvider{models: []string{"gpt-4o-mini"}}, nil
		},
		now:       func() time.Time { return time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC) },
		providers: []string{"openai"},
		listItem:  func(string, string) { t.Fatal("did not expect human list output in json mode") },
	})
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"list", "--json"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models list failed: %v", err)
	}
	var payload struct {
		Models []struct {
			Model     string `json:"model"`
			Provider  string `json:"provider"`
			Transport string `json:"transport"`
			Cached    bool   `json:"cached"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &payload); err != nil {
		t.Fatalf("json decode failed: %v\npayload=%s", err, stdout.String())
	}
	if len(payload.Models) != 1 || payload.Models[0].Model != "gpt-4o-mini" || payload.Models[0].Provider != "openai" || payload.Models[0].Transport != "api" || payload.Models[0].Cached {
		t.Fatalf("unexpected json payload: %+v", payload.Models)
	}
}

func TestModelsListUsesCacheUnlessRefreshed(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	buildCalls := 0
	newCommand := func() *cobra.Command {
		return newModelsCommandWithDeps(modelsCommandDeps{
			loadConfig:    func() (config.Config, error) { return config.Default(), nil },
			resolveAPIKey: func(string) (string, error) { return "key", nil },
			buildProvider: func(string, string, string, time.Duration) (provider.Provider, error) {
				buildCalls++
				return modelsTestProvider{models: []string{"gpt-4o-mini"}}, nil
			},
			cacheDir:  func() string { return cacheDir },
			now:       func() time.Time { return now },
			providers: []string{"openai"},
			listItem:  func(string, string) {},
		})
	}

	cmd := newCommand()
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first models list failed: %v", err)
	}

	cmd = newCommand()
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second models list failed: %v", err)
	}

	cmd = newCommand()
	cmd.SetArgs([]string{"list", "--refresh"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("refresh models list failed: %v", err)
	}

	if buildCalls != 2 {
		t.Fatalf("expected cache hit between runs and refresh bypass, got %d build calls", buildCalls)
	}
}
