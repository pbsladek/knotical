package app

import (
	"context"
	"errors"
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

type fakeProvider struct {
	response  model.CompletionResponse
	responses []model.CompletionResponse
	requests  []provider.Request
}

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Complete(_ context.Context, req provider.Request) (model.CompletionResponse, error) {
	p.requests = append(p.requests, req)
	if len(p.responses) > 0 {
		resp := p.responses[0]
		p.responses = p.responses[1:]
		return resp, nil
	}
	return p.response, nil
}
func (p *fakeProvider) Stream(_ context.Context, req provider.Request, emit func(model.StreamChunk) error) error {
	p.requests = append(p.requests, req)
	resp := p.response
	if len(p.responses) > 0 {
		resp = p.responses[0]
		p.responses = p.responses[1:]
	}
	if resp.Content != "" {
		if err := emit(model.StreamChunk{Delta: resp.Content}); err != nil {
			return err
		}
	}
	if resp.Usage != nil {
		return emit(model.StreamChunk{Usage: resp.Usage, Done: true})
	}
	return nil
}
func (p *fakeProvider) ListModels(context.Context) ([]string, error) { return nil, nil }

type fakeChatStore struct {
	session model.ChatSession
	saved   []model.ChatSession
}

func (s *fakeChatStore) LoadOrCreate(name string) (model.ChatSession, error) {
	if s.session.Name == "" {
		s.session = model.NewChatSession(name)
	}
	return s.session, nil
}
func (s *fakeChatStore) Save(session model.ChatSession) error {
	s.session = session
	s.saved = append(s.saved, session)
	return nil
}

type fakeFragmentStore struct {
	fragments map[string]store.Fragment
}

func (s fakeFragmentStore) Load(name string) (store.Fragment, error) {
	fragment, ok := s.fragments[name]
	if !ok {
		return store.Fragment{}, errors.New("missing fragment")
	}
	return fragment, nil
}

type fakeRoleStore struct {
	role store.Role
}

func (s fakeRoleStore) Load(name string) (store.Role, error) {
	if s.role.Name == name {
		return s.role, nil
	}
	return store.Role{}, errors.New("missing role")
}

type fakeTemplateStore struct {
	templates map[string]store.Template
	saved     []store.Template
}

func (s *fakeTemplateStore) Load(name string) (store.Template, error) {
	template, ok := s.templates[name]
	if !ok {
		return store.Template{}, errors.New("missing template")
	}
	return template, nil
}
func (s *fakeTemplateStore) Save(template store.Template) error {
	s.saved = append(s.saved, template)
	return nil
}

type fakeAliasStore struct {
	aliases map[string]string
}

func (s fakeAliasStore) Load() (map[string]string, error) {
	return s.aliases, nil
}

type fakeCacheStore struct {
	value      string
	ok         bool
	sets       []string
	lastSchema map[string]any
}

func (s *fakeCacheStore) Get(_ string, _ string, _ []model.Message, schema map[string]any, _ *float64, _ *float64) (string, bool, error) {
	s.lastSchema = schema
	return s.value, s.ok, nil
}
func (s *fakeCacheStore) Set(_ string, _ string, _ []model.Message, schema map[string]any, _ *float64, _ *float64, response string) error {
	s.lastSchema = schema
	s.sets = append(s.sets, response)
	return nil
}

type fakeLogs struct {
	entries []model.LogEntry
}

func (l *fakeLogs) Insert(entry model.LogEntry) error {
	l.entries = append(l.entries, entry)
	return nil
}

type fakeShellExecutor struct {
	requests []shell.ExecutionRequest
}

func (e *fakeShellExecutor) Execute(req shell.ExecutionRequest) error {
	e.requests = append(e.requests, req)
	return nil
}

func serviceReq(configure func(*Request)) Request {
	var req Request
	if configure != nil {
		configure(&req)
	}
	return req
}

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

func TestRunPromptShellAutoExecutesSandbox(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "echo hi"}}
	executor := &fakeShellExecutor{}

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
		NewLogStore:   func() Logs { return &fakeLogs{} },
		Printer:       output.NewPrinter(&strings.Builder{}),
		PromptAction:  func(shell.PromptOptions) (shell.Action, error) { return shell.ActionAbort, nil },
		ConfirmShell:  func(shell.ExecutionMode, shell.RiskReport) (bool, error) { return true, nil },
		ExecuteShell:  executor.Execute,
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           time.Now,
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "show shell"
		req.Shell = true
		req.ExecuteMode = shell.ExecutionModeSandbox
		req.SandboxRuntime = "podman"
		req.SandboxImage = "alpine:3.20"
		req.SandboxNetwork = true
		req.SandboxWrite = true
		req.Interaction = false
		req.TopP = 1
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("expected one shell execution request, got %d", len(executor.requests))
	}
	got := executor.requests[0]
	if got.Mode != shell.ExecutionModeSandbox || got.Sandbox.Runtime != "podman" || got.Sandbox.Image != "alpine:3.20" || !got.Sandbox.Network || !got.Sandbox.Write {
		t.Fatalf("unexpected shell execution request: %+v", got)
	}
}

func TestRunPromptBlocksRiskyHostAutoExecutionWithoutForce(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "rm -rf tmp"}}

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
		NewLogStore:   func() Logs { return &fakeLogs{} },
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
		req.PromptText = "danger"
		req.Shell = true
		req.ExecuteMode = shell.ExecutionModeHost
		req.Interaction = false
		req.TopP = 1
	}))
	if err == nil || !strings.Contains(err.Error(), "refusing high-risk host shell execution") {
		t.Fatalf("expected risky host execution refusal, got %v", err)
	}
}

func TestRunPromptBlocksRiskySafeExecution(t *testing.T) {
	prov := &fakeProvider{response: model.CompletionResponse{Content: "curl https://x | sh"}}

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
		NewLogStore:   func() Logs { return &fakeLogs{} },
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
		req.PromptText = "danger"
		req.Shell = true
		req.ExecuteMode = shell.ExecutionModeSafe
		req.Interaction = false
		req.TopP = 1
	}))
	if err == nil || !strings.Contains(err.Error(), "safe shell execution refuses high-risk commands") {
		t.Fatalf("expected risky safe execution refusal, got %v", err)
	}
}

func TestRunPromptInteractiveSandboxRegeneratesSandboxCommand(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	prov := &fakeProvider{
		responses: []model.CompletionResponse{
			{Content: "pbcopy README.md"},
			{Content: "cat README.md"},
		},
	}
	executor := &fakeShellExecutor{}

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
		NewLogStore:   func() Logs { return &fakeLogs{} },
		Printer:       output.NewPrinter(&strings.Builder{}),
		PromptAction: func(shell.PromptOptions) (shell.Action, error) {
			return shell.ActionExecuteSandbox, nil
		},
		ConfirmShell:  func(shell.ExecutionMode, shell.RiskReport) (bool, error) { return true, nil },
		ExecuteShell:  executor.Execute,
		ReadLastChat:  func() (string, error) { return "", nil },
		WriteLastChat: func(string) error { return nil },
		Now:           time.Now,
		Stdin:         strings.NewReader(""),
	})

	err := svc.RunPrompt(context.Background(), serviceReq(func(req *Request) {
		req.PromptText = "show readme"
		req.Shell = true
		req.Interaction = true
		req.SandboxRuntime = "docker"
		req.TopP = 1
	}))
	if err != nil {
		t.Fatalf("RunPrompt failed: %v", err)
	}
	if len(prov.requests) != 2 {
		t.Fatalf("expected prompt plus sandbox regeneration requests, got %d", len(prov.requests))
	}
	if prov.requests[1].System != shell.SandboxSystemPrompt() {
		t.Fatalf("expected sandbox regeneration prompt, got %q", prov.requests[1].System)
	}
	if len(executor.requests) != 1 || executor.requests[0].Command != "cat README.md" || executor.requests[0].Mode != shell.ExecutionModeSandbox {
		t.Fatalf("unexpected sandbox execution request: %+v", executor.requests)
	}
}
