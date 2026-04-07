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

func testRequest(configure func(*Request)) Request {
	var req Request
	if configure != nil {
		configure(&req)
	}
	return req
}

func testDeps(configure func(*Dependencies)) Dependencies {
	var deps Dependencies
	if configure != nil {
		configure(&deps)
	}
	return deps
}

func TestResolveModelAndSystemPrecedence(t *testing.T) {
	temperature := 0.7
	svc := New(testDeps(func(deps *Dependencies) {
		deps.RoleStore = fakeRoleStore{role: store.Role{
			Name:             "reviewer",
			SystemPrompt:     "role prompt",
			PrettifyMarkdown: true,
		}}
		deps.TemplateStore = &fakeTemplateStore{templates: map[string]store.Template{
			"saved": {
				Name:         "saved",
				Model:        "template-model",
				SystemPrompt: "template prompt",
				Temperature:  &temperature,
			},
		}}
	}))

	cfg := config.Default()
	cfg.DefaultModel = "default-model"
	cfg.Temperature = 0.2
	cfg.PrettifyMarkdown = true

	modelID, systemPrompt, gotTemp, renderMarkdown, err := svc.resolveModelAndSystem(testRequest(func(req *Request) {
		req.Template = "saved"
		req.Role = "reviewer"
		req.NoMD = true
		req.Temperature = 0
	}), cfg)
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
	svc := New(testDeps(func(deps *Dependencies) {
		deps.RoleStore = fakeRoleStore{role: store.Role{
			Name:             "reviewer",
			SystemPrompt:     "role prompt",
			PrettifyMarkdown: true,
		}}
		deps.TemplateStore = &fakeTemplateStore{templates: map[string]store.Template{
			"saved": {Name: "saved", SystemPrompt: "template prompt"},
		}}
	}))

	cfg := config.Default()
	modelID, systemPrompt, _, renderMarkdown, err := svc.resolveModelAndSystem(testRequest(func(req *Request) {
		req.Model = "user-model"
		req.System = "system override"
		req.Role = "reviewer"
		req.Template = "saved"
	}), cfg)
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
	if got := applySchemaFallbackInstruction("base", schemaValue, config.ProviderCapabilities{}); !strings.Contains(got, "Respond with valid JSON") {
		t.Fatalf("expected fallback instruction, got %q", got)
	}
	if got := applySchemaFallbackInstruction("base", schemaValue, config.ProviderCapabilities{NativeSchema: true}); got != "base" {
		t.Fatalf("expected native-schema provider to keep prompt, got %q", got)
	}
}

func TestResolveModelAndSystemUsesSandboxPromptForSandboxExecution(t *testing.T) {
	svc := New(Dependencies{})
	cfg := config.Default()

	_, systemPrompt, _, _, err := svc.resolveModelAndSystem(testRequest(func(req *Request) {
		req.Shell = true
		req.ExecuteMode = shell.ExecutionModeSandbox
	}), cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if systemPrompt != shell.SandboxSystemPrompt() {
		t.Fatalf("expected sandbox system prompt, got %q", systemPrompt)
	}
}

func TestResolveModelAndSystemUsesLogAnalysisPrompt(t *testing.T) {
	svc := New(Dependencies{})
	cfg := config.Default()

	_, systemPrompt, _, renderMarkdown, err := svc.resolveModelAndSystem(testRequest(func(req *Request) {
		req.AnalyzeLogs = true
	}), cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if !strings.Contains(systemPrompt, "operational logs") {
		t.Fatalf("expected log analysis system prompt, got %q", systemPrompt)
	}
	if renderMarkdown {
		t.Fatalf("expected markdown disabled by default for log analysis")
	}
}

func TestResolveModelAndSystemUsesConfiguredLogAnalysisSettings(t *testing.T) {
	svc := New(Dependencies{})
	cfg := config.Default()
	cfg.LogAnalysisMarkdown = true
	cfg.LogAnalysisSystemPrompt = "custom log prompt"

	_, systemPrompt, _, renderMarkdown, err := svc.resolveModelAndSystem(testRequest(func(req *Request) {
		req.AnalyzeLogs = true
	}), cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if systemPrompt != "custom log prompt" {
		t.Fatalf("expected configured log analysis prompt, got %q", systemPrompt)
	}
	if !renderMarkdown {
		t.Fatalf("expected configured markdown setting to be honored")
	}
}

func TestResolveModelAndSystemUsesExplicitProviderOverride(t *testing.T) {
	svc := New(Dependencies{})
	cfg := config.Default()
	cfg.DefaultProvider = "openai"

	modelID, _, _, _, err := svc.resolveModelAndSystem(testRequest(func(req *Request) {
		req.Provider = "gemini"
		req.Model = "custom-model"
	}), cfg)
	if err != nil {
		t.Fatalf("resolveModelAndSystem failed: %v", err)
	}
	if modelID != "custom-model" {
		t.Fatalf("expected explicit provider to preserve model, got %q", modelID)
	}

	state, err := svc.resolveRequestState(testRequest(func(req *Request) {
		req.Provider = "gemini"
		req.Model = "custom-model"
	}), cfg)
	if err != nil {
		t.Fatalf("resolveRequestState failed: %v", err)
	}
	if state.providerName != "gemini" {
		t.Fatalf("expected explicit provider override, got %q", state.providerName)
	}
}

func TestResolveModelAndSystemSupportsProviderPrefixedModel(t *testing.T) {
	svc := New(Dependencies{})
	cfg := config.Default()

	state, err := svc.resolveRequestState(testRequest(func(req *Request) {
		req.Model = "openai/gpt-4o-mini"
	}), cfg)
	if err != nil {
		t.Fatalf("resolveRequestState failed: %v", err)
	}
	if state.providerName != "openai" || state.modelID != "gpt-4o-mini" {
		t.Fatalf("unexpected prefixed model resolution: %+v", state)
	}
}

func TestApplyShellDefaultsFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.ShellExecuteMode = "sandbox"
	cfg.ShellSandboxRuntime = "podman"
	cfg.ShellSandboxImage = "ubuntu:24.04"
	cfg.ShellSandboxNetwork = true
	cfg.ShellSandboxWrite = true

	req := applyShellDefaults(testRequest(func(req *Request) {
		req.Shell = true
	}), cfg)
	if req.ExecuteMode != shell.ExecutionModeSandbox ||
		req.SandboxRuntime != "podman" ||
		req.SandboxImage != "ubuntu:24.04" ||
		!req.SandboxNetwork ||
		!req.SandboxWrite {
		t.Fatalf("unexpected shell defaults: %+v", req)
	}
}

func TestApplyInputDefaultsForAnalyzeLogs(t *testing.T) {
	cfg := config.Default()
	cfg.LogAnalysisSchema = "summary, likely_root_cause"
	cfg.DefaultLogProfile = "k8s"

	req := applyInputDefaults(testRequest(func(req *Request) {
		req.AnalyzeLogs = true
		req.StdinLabel = "input"
	}), cfg)
	if req.Schema != "summary, likely_root_cause" {
		t.Fatalf("expected log analysis schema default, got %q", req.Schema)
	}
	if req.StdinLabel != "logs" {
		t.Fatalf("expected default log stdin label, got %q", req.StdinLabel)
	}
	if req.Profile != "k8s" {
		t.Fatalf("expected default log profile, got %q", req.Profile)
	}
}

func TestRequestPipelineOptionsCollectsShorthandsAndTransforms(t *testing.T) {
	got := requestPipelineOptions(testRequest(func(req *Request) {
		req.Profile = "k8s"
		req.Transforms = []string{"include-regex:error"}
		req.NoPipeline = true
		req.Clean = true
		req.Unique = true
		req.K8s = true
	}))
	if got.Profile != "k8s" || !got.NoPipeline {
		t.Fatalf("unexpected pipeline options: %+v", got)
	}
	if len(got.Transforms) != 1 || got.Transforms[0] != "include-regex:error" {
		t.Fatalf("unexpected transforms: %+v", got.Transforms)
	}
	if len(got.Shorthands) != 3 || got.Shorthands[0] != "clean" || got.Shorthands[1] != "unique" || got.Shorthands[2] != "k8s" {
		t.Fatalf("unexpected shorthands: %+v", got.Shorthands)
	}
}

func TestBuildConfiguredProviderUsesCLITransport(t *testing.T) {
	var gotName string
	var gotCfg provider.CLIConfig
	svc := New(testDeps(func(deps *Dependencies) {
		deps.BuildCLIProvider = func(name string, cfg provider.CLIConfig) (provider.Provider, error) {
			gotName = name
			gotCfg = cfg
			return &fakeProvider{}, nil
		}
	}))

	cfg := config.Default()
	cfg.AnthropicTransport = "cli"

	prov, providerName, err := svc.buildConfiguredProvider(cfg, cfg.ProviderRuntime("anthropic"))
	if err != nil {
		t.Fatalf("buildConfiguredProvider failed: %v", err)
	}
	if prov == nil {
		t.Fatal("expected provider")
	}
	if gotName != "anthropic" {
		t.Fatalf("unexpected cli provider build name: %q", gotName)
	}
	if providerName != "anthropic" {
		t.Fatalf("unexpected provider name: %q", providerName)
	}
	if gotCfg.Command != "claude" {
		t.Fatalf("unexpected cli provider config: %+v", gotCfg)
	}
}

func TestLoadSessionDoesNotDuplicateSystemPrompt(t *testing.T) {
	chats := &fakeChatStore{session: model.ChatSession{
		Name: "demo",
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: "persisted"},
		},
	}}
	svc := New(testDeps(func(deps *Dependencies) {
		deps.ChatStore = chats
	}))

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

	svc := New(testDeps(func(deps *Dependencies) {
		deps.LoadConfig = func() (config.Config, error) {
			cfg := config.Default()
			cfg.DefaultModel = "gpt-4o-mini"
			cfg.DefaultProvider = "openai"
			cfg.Stream = false
			return cfg, nil
		}
		deps.ResolveAPIKey = func(string) (string, error) { return "key", nil }
		deps.BuildProvider = func(string, string, string, time.Duration) (provider.Provider, error) { return prov, nil }
		deps.ChatStore = chats
		deps.FragmentStore = fakeFragmentStore{fragments: map[string]store.Fragment{}}
		deps.RoleStore = fakeRoleStore{}
		deps.TemplateStore = &fakeTemplateStore{templates: map[string]store.Template{}}
		deps.AliasStore = fakeAliasStore{aliases: map[string]string{}}
		deps.CacheStore = &fakeCacheStore{}
		deps.NewLogStore = func() Logs { return &fakeLogs{} }
		deps.Printer = output.NewPrinter(&strings.Builder{})
		deps.ExecuteShell = func(shell.ExecutionRequest) error { return nil }
		deps.ReadLastChat = func() (string, error) { return "", nil }
		deps.WriteLastChat = func(name string) error { lastChat = name; return nil }
		deps.Now = time.Now
		deps.Stdin = strings.NewReader("hello\nexit\n")
	}))

	if err := svc.RunRepl(context.Background(), testRequest(func(req *Request) {
		req.Repl = "demo"
		req.TopP = 1
	})); err != nil {
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
