package modelcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/provider"
)

type Entry struct {
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	Transport string `json:"transport"`
	Cached    bool   `json:"cached,omitempty"`
}

type Result struct {
	Entries     []Entry
	HardErrors  []string
	MissingKeys []string
	Unsupported []string
}

type DiscoveryRequest struct {
	Config         config.Config
	Providers      []string
	ProviderFilter string
	Refresh        bool
}

type DiscoveryDeps struct {
	ResolveAPIKey    func(string) (string, error)
	BuildProvider    func(string, string, string, time.Duration) (provider.Provider, error)
	BuildCLIProvider func(string, provider.CLIConfig) (provider.Provider, error)
	CacheDir         func() string
	Now              func() time.Time
}

type cacheEntry struct {
	Transport string    `json:"transport"`
	BaseURL   string    `json:"base_url"`
	FetchedAt time.Time `json:"fetched_at"`
	Models    []string  `json:"models"`
}

const CacheTTL = time.Hour

var ErrMissingProviderKey = errors.New("missing provider key")

func Discover(ctx context.Context, req DiscoveryRequest, deps DiscoveryDeps) (Result, error) {
	if req.ProviderFilter != "" && !provider.IsKnownProvider(req.ProviderFilter) {
		return Result{}, fmt.Errorf("--provider must be openai, anthropic, gemini, or ollama")
	}
	now := time.Now().UTC()
	if deps.Now != nil {
		now = deps.Now()
	}
	result := Result{}
	for _, name := range filteredProviders(req.Providers, req.ProviderFilter) {
		if err := discoverProviderModels(ctx, req, deps, name, now, &result); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func filteredProviders(providers []string, providerFilter string) []string {
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

func discoverProviderModels(ctx context.Context, req DiscoveryRequest, deps DiscoveryDeps, name string, now time.Time, result *Result) error {
	runtimeCfg := req.Config.ProviderRuntime(name)
	if !runtimeCfg.Capabilities.ModelListing {
		result.Unsupported = append(result.Unsupported, fmt.Sprintf("%s (%s)", name, runtimeCfg.Transport))
		return nil
	}
	if !req.Refresh {
		if models, ok := loadCache(deps, name, runtimeCfg, now); ok {
			appendEntries(result, name, runtimeCfg.Transport, models, true)
			return nil
		}
	}
	prov, err := buildProvider(req.Config, deps, name, runtimeCfg)
	if err != nil {
		if errors.Is(err, ErrMissingProviderKey) {
			result.MissingKeys = append(result.MissingKeys, name)
			return nil
		}
		result.HardErrors = append(result.HardErrors, fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	models, err := prov.ListModels(ctx)
	if err != nil {
		if errors.Is(err, provider.ErrModelListingUnsupported) {
			result.Unsupported = append(result.Unsupported, fmt.Sprintf("%s (%s)", name, runtimeCfg.Transport))
			return nil
		}
		result.HardErrors = append(result.HardErrors, fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	saveCache(deps, name, runtimeCfg, now, models)
	appendEntries(result, name, runtimeCfg.Transport, models, false)
	return nil
}

func buildProvider(cfg config.Config, deps DiscoveryDeps, name string, runtimeCfg config.ProviderRuntime) (provider.Provider, error) {
	if runtimeCfg.Transport == "cli" {
		return deps.BuildCLIProvider(name, provider.CLIConfig(runtimeCfg.CLI))
	}
	key, err := deps.ResolveAPIKey(name)
	if err != nil {
		return nil, ErrMissingProviderKey
	}
	return deps.BuildProvider(name, key, runtimeCfg.BaseURL, cfg.ProviderSettings().RequestTimeout)
}

func appendEntries(result *Result, providerName string, transport string, models []string, cached bool) {
	for _, name := range models {
		result.Entries = append(result.Entries, Entry{
			Model:     name,
			Provider:  providerName,
			Transport: transport,
			Cached:    cached,
		})
	}
}

func (r Result) ListedAny() bool {
	return len(r.Entries) > 0
}

func (r Result) Warnings() []string {
	warnings := make([]string, 0, len(r.HardErrors)+len(r.Unsupported))
	warnings = append(warnings, r.HardErrors...)
	for _, name := range r.Unsupported {
		warnings = append(warnings, fmt.Sprintf("%s: model listing is not supported for this transport", name))
	}
	return warnings
}

func (r Result) FinalError() error {
	if r.ListedAny() {
		return nil
	}
	if len(r.HardErrors) > 0 {
		return fmt.Errorf("models list failed: %s", strings.Join(r.HardErrors, "; "))
	}
	if len(r.Unsupported) > 0 && len(r.MissingKeys) == 0 {
		return fmt.Errorf("models list is not supported for configured CLI transports: %s", strings.Join(r.Unsupported, ", "))
	}
	if len(r.MissingKeys) > 0 {
		return fmt.Errorf("no providers configured; missing API keys for: %s", strings.Join(r.MissingKeys, ", "))
	}
	return fmt.Errorf("no models available")
}

func loadCache(deps DiscoveryDeps, providerName string, runtimeCfg config.ProviderRuntime, now time.Time) ([]string, bool) {
	cacheDir := ""
	if deps.CacheDir != nil {
		cacheDir = deps.CacheDir()
	}
	if cacheDir == "" {
		return nil, false
	}
	payload, err := os.ReadFile(filepath.Join(cacheDir, "models-"+providerName+".json"))
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return nil, false
	}
	if entry.Transport != runtimeCfg.Transport || entry.BaseURL != runtimeCfg.BaseURL || now.Sub(entry.FetchedAt) > CacheTTL {
		return nil, false
	}
	return append([]string(nil), entry.Models...), true
}

func saveCache(deps DiscoveryDeps, providerName string, runtimeCfg config.ProviderRuntime, now time.Time, models []string) {
	cacheDir := ""
	if deps.CacheDir != nil {
		cacheDir = deps.CacheDir()
	}
	if cacheDir == "" {
		return
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return
	}
	payload, err := json.MarshalIndent(cacheEntry{
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
