package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pbsladek/knotical/internal/model"
)

type CLIConfig struct {
	Command    string
	Args       []string
	ModelFlag  string
	SystemFlag string
	SchemaFlag string
}

func BuildCLI(name string, cfg CLIConfig) (Provider, error) {
	return newCLIProvider(name, cfg, runCLICommand)
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
