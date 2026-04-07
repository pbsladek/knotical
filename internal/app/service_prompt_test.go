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

func TestRunPromptUsesAppSeams(t *testing.T) {
	prov := &fakeProvider{
		response: model.CompletionResponse{
			Content: "hello back",
			Model:   "gpt-4o-mini",
			Usage:   &model.TokenUsage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		},
	}
	chats := &fakeChatStore{}
	templates := &fakeTemplateStore{}
	cache := &fakeCacheStore{}
	logs := &fakeLogs{}
	executor := &fakeShellExecutor{}
	var lastChat string

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "alias-model"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     chats,
		FragmentStore: fakeFragmentStore{fragments: map[string]store.Fragment{"ctx": {Name: "ctx", Content: "fragment body"}}},
		RoleStore: fakeRoleStore{role: store.Role{
			Name:             "reviewer",
			SystemPrompt:     "be terse",
			PrettifyMarkdown: true,
		}},
		TemplateStore: templates,
		AliasStore:    fakeAliasStore{aliases: map[string]string{"alias-model": "gpt-4o-mini"}},
		CacheStore:    cache,
		NewLogStore: func() Logs {
			return logs
		},
		Printer:       output.NewPrinter(&strings.Builder{}),
		PromptAction:  func(shell.PromptOptions) (shell.Action, error) { return shell.ActionAbort, nil },
		ConfirmShell:  func(shell.ExecutionMode, shell.RiskReport) (bool, error) { return true, nil },
		ExecuteShell:  executor.Execute,
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(name string) error { lastChat = name; return nil },
		Now:           func() time.Time { return time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC) },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "hello"
		req.Fragments = []string{"ctx"}
		req.Role = "reviewer"
		req.Chat = "demo"
		req.TopP = 1
		req.Cache = true
		req.Interaction = true
		req.Save = "saved"
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(prov.requests))
	}
	req := prov.requests[0]
	if req.Model != "gpt-4o-mini" {
		t.Fatalf("expected aliased model, got %q", req.Model)
	}
	if req.System != "be terse" {
		t.Fatalf("expected role system prompt, got %q", req.System)
	}
	if got := req.Messages[len(req.Messages)-1].Content; got != "hello\n\nfragment body" {
		t.Fatalf("expected fragment-appended prompt, got %q", got)
	}
	if len(chats.saved) != 1 || len(chats.saved[0].Messages) != 3 {
		t.Fatalf("expected saved chat with system+turn, got %+v", chats.saved)
	}
	if lastChat != "demo" {
		t.Fatalf("expected last chat write, got %q", lastChat)
	}
	if len(logs.entries) != 1 || logs.entries[0].InputTokens == nil || *logs.entries[0].InputTokens != 5 {
		t.Fatalf("expected logged usage, got %+v", logs.entries)
	}
	if logs.entries[0].FragmentsJSON == nil || !strings.Contains(*logs.entries[0].FragmentsJSON, `"ctx"`) {
		t.Fatalf("expected logged fragments, got %+v", logs.entries[0])
	}
	if len(templates.saved) != 1 || templates.saved[0].Name != "saved" {
		t.Fatalf("expected saved template, got %+v", templates.saved)
	}
}

func TestRunPromptUsesCacheWithoutProviderCall(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "miss"}}
	cache := &fakeCacheStore{value: "cached", ok: true}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
		FragmentStore: fakeFragmentStore{fragments: map[string]store.Fragment{}},
		RoleStore:     fakeRoleStore{},
		TemplateStore: &fakeTemplateStore{templates: map[string]store.Template{}},
		AliasStore:    fakeAliasStore{aliases: map[string]string{}},
		CacheStore:    cache,
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

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "hello"
		req.TopP = 1
		req.Cache = true
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}
	if len(prov.requests) != 0 {
		t.Fatalf("expected cache hit to skip provider, got %d requests", len(prov.requests))
	}
}

func TestRunPromptAppliesConfiguredInputReduction(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "ok"}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			cfg.MaxInputLines = 2
			cfg.DefaultTailLines = 3
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "analyze"
		req.StdinText = "l1\nl2\nl3\nl4"
		req.TopP = 1
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(prov.requests))
	}
	got := prov.requests[0].Messages[len(prov.requests[0].Messages)-1].Content
	want := "analyze\n\ninput:\nl2\nl3"
	if got != want {
		t.Fatalf("unexpected reduced prompt:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRunPromptFailsOnConfiguredTokenBudget(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "ok"}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			cfg.MaxInputTokens = 2
			cfg.InputReductionMode = "fail"
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "analyze"
		req.StdinText = "abcdefghijklmno"
		req.TopP = 1
	}))
	if err == nil || err.Error() != "input exceeds max token budget: estimated 4 > 2" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prov.requests) != 0 {
		t.Fatalf("expected no provider request on budget failure, got %d", len(prov.requests))
	}
}

func TestRunPromptAnalyzeLogsAppliesSchemaAndLogLabel(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: `{"summary":"ok","likely_root_cause":"db","next_steps":"restart"}`}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			cfg.LogAnalysisSchema = "summary, likely_root_cause, next_steps"
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "find the root cause"
		req.StdinText = "error line one\nerror line two"
		req.AnalyzeLogs = true
		req.TopP = 1
		req.Interaction = true
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(prov.requests))
	}
	req := prov.requests[0]
	if !strings.Contains(req.System, "operational logs") {
		t.Fatalf("expected log analysis system prompt, got %q", req.System)
	}
	if req.Schema == nil {
		t.Fatalf("expected configured log analysis schema")
	}
	if got := req.Messages[len(req.Messages)-1].Content; got != "find the root cause\n\nlogs:\nerror line one\nerror line two" {
		t.Fatalf("unexpected log analysis prompt body: %q", got)
	}
}

func TestRunPromptAnalyzeLogsAppliesConfiguredDefaultProfileEndToEnd(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "ok"}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			cfg.DefaultLogProfile = "k8s"
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "find the root cause"
		req.StdinText = "2026-04-04T10:00:00Z pod/api-1234567890-abcde error\n2026-04-04T10:01:00Z pod/api-abcdef1234-fghij error\n"
		req.AnalyzeLogs = true
		req.TopP = 1
		req.Interaction = true
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(prov.requests))
	}
	got := prov.requests[0].Messages[len(prov.requests[0].Messages)-1].Content
	want := "find the root cause\n\nlogs:\n[x2] pod/api-<pod> error"
	if got != want {
		t.Fatalf("expected configured default profile to shape prompt:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRunPromptRejectsConflictingPipelineShorthandsBeforeProviderCall(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "ok"}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "find the root cause"
		req.StdinText = "error line one\nerror line two"
		req.AnalyzeLogs = true
		req.Dedupe = true
		req.Unique = true
		req.TopP = 1
		req.Interaction = true
	}))
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("expected pipeline conflict error, got %v", err)
	}
	if len(prov.requests) != 0 {
		t.Fatalf("expected no provider request on pipeline conflict, got %d", len(prov.requests))
	}
}

func TestRunPromptRejectsInvalidRegexTransformBeforeProviderCall(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "ok"}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "find the root cause"
		req.StdinText = "error line one\nerror line two"
		req.AnalyzeLogs = true
		req.Transforms = []string{"include-regex:("}
		req.TopP = 1
		req.Interaction = true
	}))
	if err == nil || !strings.Contains(err.Error(), "invalid regex") {
		t.Fatalf("expected invalid regex error, got %v", err)
	}
	if len(prov.requests) != 0 {
		t.Fatalf("expected no provider request on invalid regex, got %d", len(prov.requests))
	}
}

func TestRunPromptRejectsUnknownProfileBeforeProviderCall(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "ok"}}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = false
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
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
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "find the root cause"
		req.StdinText = "error line one\nerror line two"
		req.AnalyzeLogs = true
		req.Profile = "mystery"
		req.TopP = 1
		req.Interaction = true
	}))
	if err == nil || !strings.Contains(err.Error(), `unknown log profile "mystery"`) {
		t.Fatalf("expected unknown profile error, got %v", err)
	}
	if len(prov.requests) != 0 {
		t.Fatalf("expected no provider request on unknown profile, got %d", len(prov.requests))
	}
}

func TestRunPromptSummarizesOversizedInputAndLogsReduction(t *testing.T) {
	prov := &fakeProvider{responses: []model.CompletionResponse{
		{Content: "chunk a"},
		{Content: "chunk b"},
		{Content: "merged"},
		{Content: "final answer"},
	}}
	logs := &fakeLogs{}

	svc := New(Dependencies{
		LoadConfig: func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			cfg.LogToDB = true
			cfg.SummarizeChunkOverlapLines = 0
			return cfg, nil
		},
		ResolveAPIKey: func(string) (string, error) { return "key", nil },
		BuildProvider: func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil },
		ChatStore:     &fakeChatStore{},
		FragmentStore: fakeFragmentStore{fragments: map[string]store.Fragment{}},
		RoleStore:     fakeRoleStore{},
		TemplateStore: &fakeTemplateStore{templates: map[string]store.Template{}},
		AliasStore:    fakeAliasStore{aliases: map[string]string{}},
		CacheStore:    &fakeCacheStore{},
		NewLogStore: func() Logs {
			return logs
		},
		Printer:       output.NewPrinter(&strings.Builder{}),
		PromptAction:  func(shell.PromptOptions) (shell.Action, error) { return shell.ActionAbort, nil },
		ConfirmShell:  func(shell.ExecutionMode, shell.RiskReport) (bool, error) { return true, nil },
		ExecuteShell:  func(shell.ExecutionRequest) error { return nil },
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           func() time.Time { return time.Now().UTC() },
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "analyze these logs"
		req.StdinText = "abcdefghijklmnopqrst\nuvwxyzabcdefghijklmn"
		req.StdinMode = "append"
		req.MaxInputTokens = 4
		req.InputReduction = "summarize"
		req.SummarizeChunkTokens = 8
		req.TopP = 1
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}

	if len(prov.requests) != 4 {
		t.Fatalf("expected four provider requests, got %d", len(prov.requests))
	}
	if got := prov.requests[3].Messages[len(prov.requests[3].Messages)-1].Content; got != "analyze these logs\n\ninput:\nmerged" {
		t.Fatalf("unexpected final prompt after summarization: %q", got)
	}
	if len(logs.entries) != 1 || logs.entries[0].ReductionJSON == nil {
		t.Fatalf("expected reduction metadata to be logged, got %+v", logs.entries)
	}
	if !strings.Contains(*logs.entries[0].ReductionJSON, `"summarized":true`) {
		t.Fatalf("expected summarized reduction metadata, got %s", *logs.entries[0].ReductionJSON)
	}
}

func TestSplitSummaryChunksUsesOverlap(t *testing.T) {
	text := strings.Join([]string{
		"aaaaaaa1",
		"bbbbbbb2",
		"ccccccc3",
	}, "\n")

	chunks := splitSummaryChunks(text, 8, 1)
	if len(chunks) != 2 {
		t.Fatalf("expected two chunks, got %d: %+v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[1], "bbbbbbb2") || !strings.Contains(chunks[1], "ccccccc3") {
		t.Fatalf("expected overlap between chunk 1 and 2, got %+v", chunks)
	}
}

func TestComposeSummarizedPromptPreservesInstruction(t *testing.T) {
	got := composeSummarizedPrompt(serviceReq(func(req *Request) {
		req.PromptText = "analyze"
		req.StdinMode = "append"
	}), &model.ReductionMetadata{StdinLabel: "logs"}, "summary")
	if got != "analyze\n\nlogs:\nsummary" {
		t.Fatalf("unexpected composed summarized prompt: %q", got)
	}
}

func TestLogPromptResultIncludesSchemaAndFragments(t *testing.T) {
	logs := &fakeLogs{}
	svc := New(Dependencies{
		NewLogStore: func() Logs { return logs },
		Now:         func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
	})

	err := svc.logPromptResult(logPromptResultInput{
		ModelID:      "gpt-4o-mini",
		ProviderName: "openai",
		PromptText:   "hello",
		ResponseText: "world",
		SystemPrompt: "be terse",
		SchemaValue: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		Fragments: []string{"ctx", "readme"},
	})
	if err != nil {
		t.Fatalf("logPromptResult failed: %v", err)
	}
	if len(logs.entries) != 1 {
		t.Fatalf("expected one log entry, got %+v", logs.entries)
	}
	if logs.entries[0].SchemaJSON == nil || !strings.Contains(*logs.entries[0].SchemaJSON, `"name"`) {
		t.Fatalf("expected schema json, got %+v", logs.entries[0])
	}
	if logs.entries[0].FragmentsJSON == nil || !strings.Contains(*logs.entries[0].FragmentsJSON, `"ctx"`) {
		t.Fatalf("expected fragments json, got %+v", logs.entries[0])
	}
}
