package modelcatalog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
)

type testProvider struct {
	models []string
	err    error
}

func (p testProvider) Name() string { return "test" }
func (p testProvider) Complete(context.Context, provider.Request) (model.CompletionResponse, error) {
	return model.CompletionResponse{}, nil
}
func (p testProvider) Stream(context.Context, provider.Request, func(model.StreamChunk) error) error {
	return nil
}
func (p testProvider) ListModels(context.Context) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.models, nil
}

func TestDiscoverFiltersProvider(t *testing.T) {
	var built []string
	cfg := config.Default()
	result, err := Discover(context.Background(), DiscoveryRequest{
		Config:         cfg,
		Providers:      []string{"openai", "anthropic"},
		ProviderFilter: "openai",
	}, DiscoveryDeps{
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(name, _ string, _ string, _ time.Duration) (provider.Provider, error) {
			built = append(built, name)
			return testProvider{models: []string{"gpt-4o-mini"}}, nil
		},
		BuildCLIProvider: func(string, provider.CLIConfig) (provider.Provider, error) {
			return nil, errors.New("unexpected cli build")
		},
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(built) != 1 || built[0] != "openai" {
		t.Fatalf("unexpected built providers: %+v", built)
	}
	if len(result.Entries) != 1 || result.Entries[0].Provider != "openai" {
		t.Fatalf("unexpected entries: %+v", result.Entries)
	}
}

func TestDiscoverCachesResults(t *testing.T) {
	cacheDir := t.TempDir()
	buildCalls := 0
	cfg := config.Default()
	deps := DiscoveryDeps{
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) {
			buildCalls++
			return testProvider{models: []string{"gpt-4o-mini"}}, nil
		},
		BuildCLIProvider: func(string, provider.CLIConfig) (provider.Provider, error) {
			return nil, errors.New("unexpected cli build")
		},
		CacheDir: func() string { return cacheDir },
		Now:      func() time.Time { return time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC) },
	}

	for idx := 0; idx < 2; idx++ {
		result, err := Discover(context.Background(), DiscoveryRequest{
			Config:    cfg,
			Providers: []string{"openai"},
		}, deps)
		if err != nil {
			t.Fatalf("Discover failed on run %d: %v", idx, err)
		}
		if len(result.Entries) != 1 {
			t.Fatalf("unexpected entries on run %d: %+v", idx, result.Entries)
		}
	}
	if buildCalls != 1 {
		t.Fatalf("expected one build call due to cache reuse, got %d", buildCalls)
	}
}

func TestResultWarningsAndFinalError(t *testing.T) {
	result := Result{
		HardErrors:  []string{"openai: auth failed"},
		Unsupported: []string{"anthropic (cli)"},
	}
	if got := strings.Join(result.Warnings(), " | "); !strings.Contains(got, "auth failed") || !strings.Contains(got, "not supported") {
		t.Fatalf("unexpected warnings: %q", got)
	}
	err := result.FinalError()
	if err == nil || !strings.Contains(err.Error(), "auth failed") {
		t.Fatalf("unexpected final error: %v", err)
	}
}
