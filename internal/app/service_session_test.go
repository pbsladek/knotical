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

func TestRunPromptUsesPersistedSessionSystemPrompt(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "reply"}}
	chats := &fakeChatStore{session: model.ChatSession{
		Name: "demo",
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: "persisted system"},
		},
	}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "claude-sonnet-4-5"
			cfg.DefaultProvider = "anthropic"
			cfg.Stream = false
			cfg.LogToDB = false
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
		PromptAction:  func(shell.PromptOptions) (shell.Action, error) { return shell.ActionAbort, nil },
		ConfirmShell:  func(shell.ExecutionMode, shell.RiskReport) (bool, error) { return true, nil },
		ExecuteShell:  (&fakeShellExecutor{}).Execute,
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           time.Now,
		Stdin:         strings.NewReader(""),
	})

	if err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "hello"
		req.Chat = "demo"
		req.TopP = 1
	})); err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}

	if len(prov.requests) != 1 || prov.requests[0].System != "persisted system" {
		t.Fatalf("expected persisted session system prompt, got %+v", prov.requests)
	}
}

func TestRunReplTurnPersistsSession(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "reply"}}
	chats := &fakeChatStore{session: model.NewChatSession("demo")}
	var lastChat string
	logs := &fakeLogs{}
	svc := New(Dependencies{
		ChatStore: chats,
		Printer:   output.NewPrinter(&strings.Builder{}),
		Now:       func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		NewLogStore: func() Logs {
			return logs
		},
		WriteLastChat: func(name string) error {
			lastChat = name
			return nil
		},
	})

	runCtx := replRunContext{
		cfg: config.Config{
			CoreConfig: config.CoreConfig{
				Stream:  false,
				LogToDB: true,
			},
		},
		modelID:      "gpt-4o-mini",
		systemPrompt: "be terse",
		providerName: "openai",
		prov:         prov,
		session:      model.NewChatSession("demo"),
	}
	if err := svc.runReplTurn(context.Background(), &runCtx, serviceReq(func(req *Request) {
		req.Repl = "demo"
	}), "hello"); err != nil {
		t.Fatalf("runReplTurn failed: %v", err)
	}
	if lastChat != "demo" {
		t.Fatalf("expected last chat update, got %q", lastChat)
	}
	if len(chats.saved) != 1 || len(chats.saved[0].Messages) != 2 {
		t.Fatalf("expected saved session turn, got %+v", chats.saved)
	}
	if len(logs.entries) != 1 || logs.entries[0].Conversation == nil || *logs.entries[0].Conversation != "demo" {
		t.Fatalf("expected logged repl turn, got %+v", logs.entries)
	}
}

func TestResolveModelAndSystemPrefersRoleOverTemplatePrompt(t *testing.T) {
	svc := New(Dependencies{
		RoleStore: fakeRoleStore{role: store.Role{
			Name:             "reviewer",
			SystemPrompt:     "role prompt",
			PrettifyMarkdown: false,
		}},
		TemplateStore: &fakeTemplateStore{templates: map[string]store.Template{
			"tmpl": {Name: "tmpl", Model: "gpt-4.1", SystemPrompt: "template prompt"},
		}},
	})

	cfg := config.Default()
	modelID, systemPrompt, temperature, renderMarkdown, err := svc.resolveModelAndSystem(serviceReq(func(req *Request) {
		req.Role = "reviewer"
		req.Template = "tmpl"
		req.Temperature = 0
	}), cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if modelID != "gpt-4.1" {
		t.Fatalf("expected template model, got %q", modelID)
	}
	if systemPrompt != "role prompt" {
		t.Fatalf("expected role system prompt, got %q", systemPrompt)
	}
	if renderMarkdown {
		t.Fatalf("expected role markdown setting to disable markdown")
	}
	if temperature != cfg.Temperature {
		t.Fatalf("unexpected temperature: %v", temperature)
	}
}

func TestApplySchemaFallbackInstructionOnlyForNonNativeProviders(t *testing.T) {
	schemaValue := map[string]any{"type": "object"}
	if got := applySchemaFallbackInstruction("base", schemaValue, config.ProviderCapabilities{NativeSchema: true}); got != "base" {
		t.Fatalf("expected native provider to keep prompt unchanged, got %q", got)
	}
	got := applySchemaFallbackInstruction("base", schemaValue, config.ProviderCapabilities{})
	if !strings.Contains(got, "Respond with valid JSON matching this schema") {
		t.Fatalf("expected fallback schema instruction, got %q", got)
	}
}
