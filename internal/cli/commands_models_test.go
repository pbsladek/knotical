package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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
		stderr: &stderr,
	})
	cmd.SetArgs([]string{"list"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models list failed: %v", err)
	}
	if len(listed) != 1 || listed[0] != "gpt-4o-mini:openai" {
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
