package config

import (
	"strings"

	"github.com/pbsladek/knotical/internal/provider"
)

type ProviderCapabilities = provider.Capabilities

type ProviderRuntime struct {
	Name         string
	Transport    string
	BaseURL      string
	CLI          CLIProviderConfig
	Capabilities ProviderCapabilities
}

type ProviderEndpoint struct {
	Transport string
	BaseURL   string
	CLI       CLIProviderConfig
}

type CLIProviderConfig struct {
	Command    string
	Args       []string
	ModelFlag  string
	SystemFlag string
	SchemaFlag string
}

func (cfg Config) ProviderEndpoint(name string) ProviderEndpoint {
	return ProviderEndpoint{
		Transport: cfg.TransportForProvider(name),
		BaseURL:   cfg.BaseURLForProvider(name),
		CLI:       cfg.CLIConfigForProvider(name),
	}
}

func (cfg Config) ProviderRuntime(name string) ProviderRuntime {
	endpoint := cfg.ProviderEndpoint(name)
	caps := provider.CapabilitiesForTransport(name, endpoint.Transport)
	return ProviderRuntime{
		Name:      name,
		Transport: endpoint.Transport,
		BaseURL:   endpoint.BaseURL,
		CLI:       endpoint.CLI,
		Capabilities: ProviderCapabilities{
			NativeStreaming:    caps.NativeStreaming,
			NativeConversation: caps.NativeConversation,
			NativeSchema:       caps.NativeSchema,
			ModelListing:       caps.ModelListing,
		},
	}
}

func (cfg Config) BaseURLForProvider(name string) string {
	switch name {
	case "openai":
		return cfg.OpenAIBaseURL
	case "anthropic":
		return cfg.AnthropicBaseURL
	case "gemini":
		return cfg.GeminiBaseURL
	case "ollama":
		if cfg.OllamaBaseURL != "" {
			return cfg.OllamaBaseURL
		}
		return cfg.APIBaseURL
	default:
		return ""
	}
}

func (cfg Config) TransportForProvider(name string) string {
	switch name {
	case "openai":
		return normalizedTransport(cfg.OpenAITransport)
	case "anthropic":
		return normalizedTransport(cfg.AnthropicTransport)
	case "gemini":
		return normalizedTransport(cfg.GeminiTransport)
	default:
		return "api"
	}
}

func normalizedTransport(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cli":
		return "cli"
	default:
		return "api"
	}
}

func (cfg Config) CLIConfigForProvider(name string) CLIProviderConfig {
	switch name {
	case "anthropic":
		return CLIProviderConfig{
			Command:    cfg.ClaudeCLICommand,
			Args:       append([]string(nil), cfg.ClaudeCLIArgs...),
			ModelFlag:  cfg.ClaudeCLIModelFlag,
			SystemFlag: cfg.ClaudeCLISystemFlag,
			SchemaFlag: cfg.ClaudeCLISchemaFlag,
		}
	case "openai":
		return CLIProviderConfig{
			Command:    cfg.CodexCLICommand,
			Args:       append([]string(nil), cfg.CodexCLIArgs...),
			ModelFlag:  cfg.CodexCLIModelFlag,
			SystemFlag: cfg.CodexCLISystemFlag,
			SchemaFlag: cfg.CodexCLISchemaFlag,
		}
	case "gemini":
		return CLIProviderConfig{
			Command:    cfg.GeminiCLICommand,
			Args:       append([]string(nil), cfg.GeminiCLIArgs...),
			ModelFlag:  cfg.GeminiCLIModelFlag,
			SystemFlag: cfg.GeminiCLISystemFlag,
			SchemaFlag: cfg.GeminiCLISchemaFlag,
		}
	default:
		return CLIProviderConfig{}
	}
}
