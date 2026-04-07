package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
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

var ErrModelListingUnsupported = errors.New("model listing is not supported")

type CLIConfig struct {
	Command    string
	Args       []string
	ModelFlag  string
	SystemFlag string
	SchemaFlag string
}

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

func BuildCLI(name string, cfg CLIConfig) (Provider, error) {
	return newCLIProvider(name, cfg, runCLICommand)
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

type cliProvider struct {
	name string
	cfg  CLIConfig
	run  func(context.Context, string, []string) ([]byte, error)
}

func newCLIProvider(name string, cfg CLIConfig, run func(context.Context, string, []string) ([]byte, error)) (Provider, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("%s CLI transport is not configured", name)
	}
	return cliProvider{name: name, cfg: cfg, run: run}, nil
}

func (p cliProvider) Name() string { return p.name }

func (p cliProvider) Complete(ctx context.Context, req Request) (model.CompletionResponse, error) {
	args := append([]string{}, p.cfg.Args...)
	if p.cfg.ModelFlag != "" && req.Model != "" {
		args = append(args, p.cfg.ModelFlag, req.Model)
	}
	if p.cfg.SystemFlag != "" && req.System != "" {
		args = append(args, p.cfg.SystemFlag, req.System)
	}
	if p.cfg.SchemaFlag != "" && req.Schema != nil {
		payload, err := json.Marshal(req.Schema)
		if err != nil {
			return model.CompletionResponse{}, err
		}
		args = append(args, p.cfg.SchemaFlag, string(payload))
	}
	args = append(args, cliPromptText(req, p.cfg))
	output, err := p.run(ctx, p.cfg.Command, args)
	if err != nil {
		return model.CompletionResponse{}, err
	}
	return model.CompletionResponse{
		Content: strings.TrimSpace(string(output)),
		Model:   req.Model,
	}, nil
}

func (p cliProvider) Stream(ctx context.Context, req Request, emit func(model.StreamChunk) error) error {
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return err
	}
	if resp.Content != "" {
		if err := emit(model.StreamChunk{Delta: resp.Content}); err != nil {
			return err
		}
	}
	return emit(model.StreamChunk{Done: true})
}

func (p cliProvider) ListModels(context.Context) ([]string, error) {
	return nil, fmt.Errorf("%w for %s CLI transport", ErrModelListingUnsupported, p.name)
}

func cliPromptText(req Request, cfg CLIConfig) string {
	var parts []string
	if req.System != "" && cfg.SystemFlag == "" {
		parts = append(parts, "System:\n"+req.System)
	}
	if req.Schema != nil && cfg.SchemaFlag == "" {
		payload, _ := json.Marshal(req.Schema)
		parts = append(parts, fmt.Sprintf("Respond with valid JSON matching this schema: %s. No other text.", string(payload)))
	}

	if isSingleUserPrompt(req.Messages) {
		parts = append(parts, req.Messages[0].Content)
		return strings.Join(parts, "\n\n")
	}
	parts = append(parts, conversationTranscript(req.Messages))
	return strings.Join(parts, "\n\n")
}

func isSingleUserPrompt(messages []model.Message) bool {
	if len(messages) != 1 || messages[0].Role == model.RoleSystem {
		return false
	}
	for _, msg := range messages {
		if msg.Role == model.RoleAssistant {
			return false
		}
	}
	return true
}

func conversationTranscript(messages []model.Message) string {
	var transcript strings.Builder
	transcript.WriteString("Conversation:\n")
	for _, msg := range messages {
		switch msg.Role {
		case model.RoleSystem:
			continue
		case model.RoleAssistant:
			transcript.WriteString("Assistant: ")
		default:
			transcript.WriteString("User: ")
		}
		transcript.WriteString(msg.Content)
		transcript.WriteString("\n\n")
	}
	return strings.TrimSpace(transcript.String())
}

func runCLICommand(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, text)
	}
	return output, nil
}
