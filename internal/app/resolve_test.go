package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
	"github.com/pbsladek/knotical/internal/store"
)

func TestResolveModelAndSystemPrecedence(t *testing.T) {
	temperature := 0.7
	svc := New(Dependencies{
		RoleStore: fakeRoleStore{role: store.Role{
			Name:             "reviewer",
			SystemPrompt:     "role prompt",
			PrettifyMarkdown: true,
		}},
		TemplateStore: &fakeTemplateStore{templates: map[string]store.Template{
			"saved": {
				Name:         "saved",
				Model:        "template-model",
				SystemPrompt: "template prompt",
				Temperature:  &temperature,
			},
		}},
	})

	cfg := config.Default()
	cfg.DefaultModel = "default-model"
	cfg.Temperature = 0.2
	cfg.PrettifyMarkdown = true

	modelID, systemPrompt, gotTemp, renderMarkdown, err := svc.resolveModelAndSystem(Request{
		Template:    "saved",
		Role:        "reviewer",
		NoMD:        true,
		Temperature: 0,
	}, cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if modelID != "template-model" {
		t.Fatalf("expected template model, got %q", modelID)
	}
	if systemPrompt != "role prompt" {
		t.Fatalf("expected role system prompt, got %q", systemPrompt)
	}
	if gotTemp != 0.7 {
		t.Fatalf("expected template temperature, got %v", gotTemp)
	}
	if renderMarkdown {
		t.Fatalf("expected no markdown rendering")
	}
}

func TestResolveModelAndSystemSystemOverridesRoleAndTemplate(t *testing.T) {
	svc := New(Dependencies{
		RoleStore: fakeRoleStore{role: store.Role{
			Name:             "reviewer",
			SystemPrompt:     "role prompt",
			PrettifyMarkdown: true,
		}},
		TemplateStore: &fakeTemplateStore{templates: map[string]store.Template{
			"saved": {Name: "saved", SystemPrompt: "template prompt"},
		}},
	})

	cfg := config.Default()
	modelID, systemPrompt, _, renderMarkdown, err := svc.resolveModelAndSystem(Request{
		Model:    "user-model",
		System:   "system override",
		Role:     "reviewer",
		Template: "saved",
	}, cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if modelID != "user-model" {
		t.Fatalf("expected user model, got %q", modelID)
	}
	if systemPrompt != "system override" {
		t.Fatalf("expected system override, got %q", systemPrompt)
	}
	if !renderMarkdown {
		t.Fatalf("expected markdown rendering to remain enabled")
	}
}

func TestApplySchemaFallbackInstruction(t *testing.T) {
	schemaValue := map[string]any{"type": "object"}
	if got := applySchemaFallbackInstruction("base", schemaValue, "anthropic"); !strings.Contains(got, "Respond with valid JSON") {
		t.Fatalf("expected fallback instruction, got %q", got)
	}
	if got := applySchemaFallbackInstruction("base", schemaValue, "openai"); got != "base" {
		t.Fatalf("expected native-schema provider to keep prompt, got %q", got)
	}
}

func TestResolveModelAndSystemUsesSandboxPromptForSandboxExecution(t *testing.T) {
	svc := New(Dependencies{})
	cfg := config.Default()

	_, systemPrompt, _, _, err := svc.resolveModelAndSystem(Request{
		Shell:       true,
		ExecuteMode: shell.ExecutionModeSandbox,
	}, cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if systemPrompt != shell.SandboxSystemPrompt() {
		t.Fatalf("expected sandbox system prompt, got %q", systemPrompt)
	}
}

func TestLoadSessionDoesNotDuplicateSystemPrompt(t *testing.T) {
	chats := &fakeChatStore{session: model.ChatSession{
		Name: "demo",
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: "persisted"},
		},
	}}
	svc := New(Dependencies{ChatStore: chats})

	session, err := svc.loadSession("demo", "ignored")
	if err != nil {
		t.Fatalf("loadSession failed: %v", err)
	}
	if len(session.Messages) != 1 {
		t.Fatalf("expected single system message, got %+v", session.Messages)
	}
	if session.Messages[0].Content != "ignored" {
		t.Fatalf("expected system prompt override to persist, got %+v", session.Messages)
	}
}

func TestRunReplPersistsTurns(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "pong"}}
	chats := &fakeChatStore{}
	var lastChat string

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     chats,
		FragmentStore: fakeFragmentStore{fragments: map[string]store.Fragment{}},
		RoleStore:     fakeRoleStore{},
		TemplateStore: &fakeTemplateStore{templates: map[string]store.Template{}},
		AliasStore:    fakeAliasStore{aliases: map[string]string{}},
		CacheStore:    &fakeCacheStore{},
		NewLogStore: func() Logs {
			return &fakeLogs{}
		},
		Printer:       output.NewPrinter(&strings.Builder{}),
		PromptAction:  nil,
		ConfirmShell:  nil,
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(name string) error { lastChat = name; return nil },
		Now:           time.Now,
		Stdin:         strings.NewReader("hello\nexit\n"),
	})

	if err := svc.RunRepl(context.Background(), Request{Repl: "demo", TopP: 1}); err != nil {
		t.Fatalf("RunRepl failed: %v", err)
	}
	if len(prov.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(prov.requests))
	}
	if len(chats.saved) != 1 || len(chats.saved[0].Messages) != 2 {
		t.Fatalf("expected saved REPL transcript, got %+v", chats.saved)
	}
	if lastChat != "demo" {
		t.Fatalf("expected last chat update, got %q", lastChat)
	}
}
