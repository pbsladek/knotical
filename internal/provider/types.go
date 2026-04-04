package provider

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

type Request struct {
	Model       string
	Messages    []model.Message
	System      string
	Schema      map[string]any
	Temperature *float64
	TopP        *float64
	MaxTokens   int64
	Stream      bool
}

type Provider interface {
	Name() string
	Complete(context.Context, Request) (model.CompletionResponse, error)
	Stream(context.Context, Request, func(model.StreamChunk) error) error
	ListModels(context.Context) ([]string, error)
}

func DetectProvider(modelID string, defaultProvider string) string {
	switch {
	case strings.HasPrefix(modelID, "claude-"):
		return "anthropic"
	case strings.HasPrefix(modelID, "gpt-"), strings.HasPrefix(modelID, "o1"), strings.HasPrefix(modelID, "o3"):
		return "openai"
	case strings.HasPrefix(modelID, "ollama/"):
		return "ollama"
	case strings.HasPrefix(modelID, "gemini-"):
		return "gemini"
	default:
		return defaultProvider
	}
}

func Build(name string, apiKey string, apiBaseURL string, timeout time.Duration) (Provider, error) {
	if err := validateBaseURL(name, apiBaseURL); err != nil {
		return nil, err
	}
	switch name {
	case "openai":
		return NewOpenAIProvider(apiKey, apiBaseURL, timeout), nil
	case "anthropic":
		return NewAnthropicProvider(apiKey, apiBaseURL, timeout), nil
	case "gemini":
		return NewGeminiProvider(apiKey, apiBaseURL, timeout)
	case "ollama":
		return NewOllamaProvider(apiBaseURL, timeout), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

func validateBaseURL(providerName string, rawURL string) error {
	if rawURL == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid %s base URL: %w", providerName, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid %s base URL: unsupported scheme %q", providerName, parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid %s base URL: missing host", providerName)
	}
	if parsed.User != nil {
		return fmt.Errorf("invalid %s base URL: credentials in URL are not allowed", providerName)
	}
	if providerName != "ollama" && parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("invalid %s base URL: non-HTTPS endpoints are only allowed for localhost", providerName)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func maxTokens(value int64, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}
