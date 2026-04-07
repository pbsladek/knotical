package provider

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/model"
)

func TestCLIProviderCompleteBuildsArgsAndPrompt(t *testing.T) {
	var gotName string
	var gotArgs []string
	prov, err := newCLIProvider("anthropic", CLIConfig{
		Command:    "claude",
		Args:       []string{"-p"},
		ModelFlag:  "--model",
		SystemFlag: "--system-prompt",
		SchemaFlag: "--json-schema",
	}, func(ctx context.Context, name string, args []string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string{}, args...)
		return []byte("hello from cli\n"), nil
	})
	if err != nil {
		t.Fatalf("newCLIProvider failed: %v", err)
	}

	resp, err := prov.Complete(context.Background(), Request{
		Model:  "claude-sonnet-4-5",
		System: "be terse",
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "say hi"},
		},
		Schema: map[string]any{
			"type": "object",
		},
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if gotName != "claude" {
		t.Fatalf("unexpected command: %q", gotName)
	}
	wantArgs := []string{
		"-p",
		"--model", "claude-sonnet-4-5",
		"--system-prompt", "be terse",
		"--json-schema", `{"type":"object"}`,
		"say hi",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if resp.Content != "hello from cli" {
		t.Fatalf("unexpected response: %q", resp.Content)
	}
	if resp.Model != "claude-sonnet-4-5" {
		t.Fatalf("unexpected model: %q", resp.Model)
	}
}

func TestCLIProviderCompleteFallsBackToPromptInjection(t *testing.T) {
	var gotPrompt string
	prov, err := newCLIProvider("gemini", CLIConfig{
		Command:   "gemini",
		Args:      []string{"-p"},
		ModelFlag: "--model",
	}, func(ctx context.Context, name string, args []string) ([]byte, error) {
		gotPrompt = args[len(args)-1]
		return []byte("ok"), nil
	})
	if err != nil {
		t.Fatalf("newCLIProvider failed: %v", err)
	}

	_, err = prov.Complete(context.Background(), Request{
		Model:  "gemini-2.5-pro",
		System: "be terse",
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "summarize"},
		},
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if gotPrompt == "" || gotPrompt == "summarize" {
		t.Fatalf("expected prompt injection, got %q", gotPrompt)
	}
}

func TestCLIProviderFormatsConversationTranscript(t *testing.T) {
	prompt := cliPromptText(Request{
		System: "be terse",
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: "ignored here"},
			{Role: model.RoleUser, Content: "first"},
			{Role: model.RoleAssistant, Content: "second"},
			{Role: model.RoleUser, Content: "third"},
		},
	}, CLIConfig{})

	if prompt == "" ||
		!containsAll(prompt, "System:\nbe terse", "Conversation:", "User: first", "Assistant: second", "User: third") {
		t.Fatalf("unexpected transcript: %q", prompt)
	}
}

func TestCLIProviderStreamFallsBackToSingleChunk(t *testing.T) {
	prov, err := newCLIProvider("openai", CLIConfig{
		Command: "codex",
		Args:    []string{"exec"},
	}, func(ctx context.Context, name string, args []string) ([]byte, error) {
		return []byte("streamed once"), nil
	})
	if err != nil {
		t.Fatalf("newCLIProvider failed: %v", err)
	}

	var deltas []string
	doneCount := 0
	if err := prov.Stream(context.Background(), Request{
		Model:    "gpt-5",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	}, func(chunk model.StreamChunk) error {
		if chunk.Delta != "" {
			deltas = append(deltas, chunk.Delta)
		}
		if chunk.Done {
			doneCount++
		}
		return nil
	}); err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	if len(deltas) != 1 || deltas[0] != "streamed once" {
		t.Fatalf("unexpected deltas: %#v", deltas)
	}
	if doneCount != 1 {
		t.Fatalf("expected one done chunk, got %d", doneCount)
	}
}

func TestCLIProviderReturnsConfigurationError(t *testing.T) {
	if _, err := newCLIProvider("anthropic", CLIConfig{}, runCLICommand); err == nil {
		t.Fatal("expected missing command to fail")
	}
}

func TestRunCLICommandIncludesStderrText(t *testing.T) {
	_, err := runCLICommand(context.Background(), "sh", []string{"-c", "echo boom >&2; exit 1"})
	if err == nil {
		t.Fatal("expected command failure")
	}
	if !containsAll(err.Error(), "boom") {
		t.Fatalf("expected stderr text in error, got %v", err)
	}
}

func TestCLIProviderBubblesExecutionErrors(t *testing.T) {
	prov, err := newCLIProvider("gemini", CLIConfig{
		Command: "gemini",
	}, func(ctx context.Context, name string, args []string) ([]byte, error) {
		return nil, errors.New("failed")
	})
	if err != nil {
		t.Fatalf("newCLIProvider failed: %v", err)
	}
	if _, err := prov.Complete(context.Background(), Request{
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	}); err == nil {
		t.Fatal("expected Complete to fail")
	}
}

func containsAll(text string, patterns ...string) bool {
	for _, pattern := range patterns {
		if !strings.Contains(text, pattern) {
			return false
		}
	}
	return true
}
