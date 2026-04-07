package provider

import (
	"fmt"
	"strings"
)

func NormalizeProviderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func IsKnownProvider(name string) bool {
	switch NormalizeProviderName(name) {
	case "openai", "anthropic", "gemini", "ollama":
		return true
	default:
		return false
	}
}

func DetectProvider(modelID string, defaultProvider string) string {
	providerName, _, err := ResolveModel(modelID, "", defaultProvider)
	if err != nil {
		return NormalizeProviderName(defaultProvider)
	}
	return providerName
}

func ResolveModel(modelID string, explicitProvider string, defaultProvider string) (string, string, error) {
	explicitProvider = NormalizeProviderName(explicitProvider)
	if explicitProvider != "" && !IsKnownProvider(explicitProvider) {
		return "", "", fmt.Errorf("unknown provider %q", explicitProvider)
	}

	modelID = strings.TrimSpace(modelID)
	if providerName, strippedModel, ok := splitProviderModel(modelID); ok {
		if strippedModel == "" {
			return "", "", fmt.Errorf("invalid model %q: missing model after provider prefix", modelID)
		}
		if explicitProvider != "" && explicitProvider != providerName {
			return "", "", fmt.Errorf("model %q selects provider %s but explicit provider %s was also set", modelID, providerName, explicitProvider)
		}
		return providerName, strippedModel, nil
	}

	if explicitProvider != "" {
		return explicitProvider, modelID, nil
	}
	if detected := detectProviderByModelID(modelID); detected != "" {
		return detected, modelID, nil
	}
	return NormalizeProviderName(defaultProvider), modelID, nil
}

func splitProviderModel(modelID string) (string, string, bool) {
	prefix, rest, ok := strings.Cut(modelID, "/")
	if !ok {
		return "", modelID, false
	}
	prefix = NormalizeProviderName(prefix)
	if !IsKnownProvider(prefix) {
		return "", modelID, false
	}
	return prefix, rest, true
}

func detectProviderByModelID(modelID string) string {
	switch {
	case strings.HasPrefix(modelID, "claude-"):
		return "anthropic"
	case strings.HasPrefix(modelID, "gpt-"), strings.HasPrefix(modelID, "o1"), strings.HasPrefix(modelID, "o3"):
		return "openai"
	case strings.HasPrefix(modelID, "gemini-"):
		return "gemini"
	default:
		return ""
	}
}
