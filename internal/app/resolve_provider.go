package app

import (
	"strings"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/store"
)

func (s *Service) appendFragments(prompt string, names []string) (string, error) {
	if len(names) == 0 {
		return prompt, nil
	}
	parts := []string{prompt}
	for _, name := range names {
		fragment, err := s.deps.FragmentStore.Load(name)
		if err != nil {
			return "", err
		}
		parts = append(parts, fragment.Content)
	}
	return strings.Join(parts, "\n\n"), nil
}

func (s *Service) resolveAlias(modelID string) string {
	if s.deps.AliasStore == nil {
		return modelID
	}
	aliases, err := s.deps.AliasStore.Load()
	if err != nil {
		return modelID
	}
	if resolved, ok := aliases[modelID]; ok {
		return resolved
	}
	return modelID
}

func (s *Service) buildConfiguredProvider(cfg config.Config, runtime config.ProviderRuntime) (provider.Provider, string, error) {
	if runtime.Transport == "cli" {
		prov, err := s.deps.BuildCLIProvider(runtime.Name, provider.CLIConfig(runtime.CLI))
		if err != nil {
			return nil, "", err
		}
		return prov, runtime.Name, nil
	}
	providerCfg := cfg.ProviderSettings()
	apiKey, err := s.deps.ResolveAPIKey(runtime.Name)
	if err != nil {
		return nil, "", err
	}
	prov, err := s.deps.BuildProvider(runtime.Name, apiKey, runtime.BaseURL, providerCfg.RequestTimeout)
	if err != nil {
		return nil, "", err
	}
	return prov, runtime.Name, nil
}

func defaultResolveAPIKey(providerName string) (string, error) {
	if providerName == "ollama" || strings.HasSuffix(providerName, "-cli") {
		return "", nil
	}
	return store.NewKeyManager(config.KeysFilePath()).Require(providerName)
}
