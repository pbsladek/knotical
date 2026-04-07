package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/store"
)

func rootReq(configure func(*app.Request)) app.Request {
	var req app.Request
	if configure != nil {
		configure(&req)
	}
	return req
}

func TestRunPromptCacheHitStillPersistsChatAndLogs(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")

	requests := 0
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"cached reply"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	session := model.NewChatSession("demo")
	session.PushSystem("be terse")
	chatStore := store.ChatStore{Dir: config.ChatCacheDir()}
	if err := chatStore.Save(session); err != nil {
		t.Fatalf("chat save failed: %v", err)
	}

	cacheStore := store.CacheStore{Dir: config.CacheDir()}
	messages := append([]model.Message{}, session.Messages...)
	messages = append(messages, model.Message{Role: model.RoleUser, Content: "hello"})
	if err := cacheStore.Set("gpt-4o-mini", "be terse", messages, nil, nil, nil, "cached reply"); err != nil {
		t.Fatalf("cache set failed: %v", err)
	}

	opts := rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.Chat = "demo"
			req.System = "be terse"
			req.Cache = true
			req.NoStream = true
			req.TopP = 1
		}),
		Prompt: []string{"hello"},
	}
	if err := runPrompt(context.Background(), opts); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}
	recorder.Assert(t)

	if requests != 0 {
		t.Fatalf("expected no provider request due to cache, got %d", requests)
	}

	session, err := chatStore.LoadOrCreate("demo")
	if err != nil {
		t.Fatalf("chat load failed: %v", err)
	}
	if len(session.Messages) != 3 {
		t.Fatalf("expected system plus cached turn, got %+v", session.Messages)
	}

	logStore := store.NewLogStore(config.LogsDBPath())
	count, err := logStore.Count()
	if err != nil {
		t.Fatalf("log count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 log entry, got %d", count)
	}

}

func TestRunPromptChatPreservesSystemPromptForLogsAndSave(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")

	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	opts := rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.Chat = "demo"
			req.System = "be terse"
			req.Save = "saved-template"
			req.NoStream = true
			req.TopP = 1
		}),
		Prompt: []string{"hello"},
	}
	if err := runPrompt(context.Background(), opts); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}
	recorder.Assert(t)

	logStore := store.NewLogStore(config.LogsDBPath())
	entries, err := logStore.Query(store.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("log query failed: %v", err)
	}
	if len(entries) != 1 || entries[0].SystemPrompt == nil || *entries[0].SystemPrompt != "be terse" {
		t.Fatalf("expected logged system prompt, got %+v", entries)
	}
	if entries[0].InputTokens == nil || *entries[0].InputTokens != 10 || entries[0].OutputTokens == nil || *entries[0].OutputTokens != 5 {
		t.Fatalf("expected logged token usage, got %+v", entries[0])
	}

	template, err := (store.TemplateStore{Dir: config.TemplatesDir()}).Load("saved-template")
	if err != nil {
		t.Fatalf("template load failed: %v", err)
	}
	if template.SystemPrompt != "be terse" {
		t.Fatalf("expected saved template system prompt, got %+v", template)
	}
}

func TestRunPromptStreamLogsTokenUsage(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")

	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			recorder.Failf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello","logprobs":[],"sequence_number":1}`,
			"",
			`data: {"type":"response.completed","sequence_number":2,"response":{"id":"resp_123","object":"response","created_at":1700000000,"status":"completed","model":"gpt-4o-mini","output":[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello"}]}],"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}}`,
			"",
			"",
		}, "\n"))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = true
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	opts := rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.Cache = false
			req.NoMD = true
			req.TopP = 1
		}),
		Prompt: []string{"hello"},
	}
	if err := runPrompt(context.Background(), opts); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}
	recorder.Assert(t)

	logStore := store.NewLogStore(config.LogsDBPath())
	entries, err := logStore.Query(store.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("log query failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].InputTokens == nil || *entries[0].InputTokens != 7 || entries[0].OutputTokens == nil || *entries[0].OutputTokens != 3 {
		t.Fatalf("expected streamed token usage in logs, got %+v", entries[0])
	}
}

func TestRunPromptNoLogOverridesEnabledConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")

	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = false
	cfg.LogToDB = true
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	if err := runPrompt(context.Background(), rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.NoLog = true
			req.NoStream = true
			req.TopP = 1
		}),
		Prompt: []string{"hello"},
	}); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}
	recorder.Assert(t)

	logStore := store.NewLogStore(config.LogsDBPath())
	count, err := logStore.Count()
	if err != nil {
		t.Fatalf("log count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no logs, got %d", count)
	}
}

func TestRunPromptLogOverridesDisabledConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")

	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = false
	cfg.LogToDB = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	if err := runPrompt(context.Background(), rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.Log = true
			req.NoStream = true
			req.TopP = 1
		}),
		Prompt: []string{"hello"},
	}); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}
	recorder.Assert(t)

	logStore := store.NewLogStore(config.LogsDBPath())
	count, err := logStore.Count()
	if err != nil {
		t.Fatalf("log count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one log entry, got %d", count)
	}
}

func TestRunPromptCombinesPromptAndStdinInProviderRequest(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")
	withTestStdin(t, "error line\nstack trace\n")

	recorder := &handlerFailureRecorder{}
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			recorder.Failf("decode request failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	if err := runPrompt(context.Background(), rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.NoStream = true
			req.TopP = 1
		}),
		Prompt: []string{"analyze", "these", "logs"},
	}); err != nil {
		t.Fatalf("runPrompt failed: %v", err)
	}
	recorder.Assert(t)

	input, ok := requestBody["input"].([]any)
	if !ok || len(input) == 0 {
		t.Fatalf("unexpected request body: %+v", requestBody)
	}
	firstItem, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first input item: %+v", input[0])
	}
	got, _ := firstItem["content"].(string)
	want := "analyze these logs\n\ninput:\nerror line\nstack trace"
	if got != want {
		t.Fatalf("unexpected composed prompt:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestValidateRootOptionsRejectsConflictingLogFlags(t *testing.T) {
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.Log = true
		req.NoLog = true
	})}); err == nil {
		t.Fatal("expected conflicting logging flags to fail")
	}
}

func TestValidateRootOptionsRejectsInvalidStdinMode(t *testing.T) {
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.StdinMode = "weird" })}); err == nil {
		t.Fatal("expected invalid stdin mode to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.InputReduction = "mystery" })}); err == nil {
		t.Fatal("expected invalid input reduction mode to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.MaxInputLines = -1 })}); err == nil {
		t.Fatal("expected negative input limit to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.MaxInputTokens = -1 })}); err == nil {
		t.Fatal("expected negative token limit to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.Shell = true
	})}); err == nil {
		t.Fatal("expected analyze-logs conflict with shell mode")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.Code = true
	})}); err == nil {
		t.Fatal("expected analyze-logs conflict with code mode")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.DescribeShell = true
	})}); err == nil {
		t.Fatal("expected analyze-logs conflict with describe-shell mode")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Profile = "k8s" })}); err == nil {
		t.Fatal("expected profile without analyze-logs to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.Profile = "k8s"
	})}); err != nil {
		t.Fatalf("expected analyze-logs profile combination to pass, got %v", err)
	}
}

func TestValidateRootOptionsRejectsNoPipelineWithExplicitPipelineFlags(t *testing.T) {
	tests := []rootOptions{
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Profile = "k8s" })},
		{Request: rootReq(func(req *app.Request) {
			req.AnalyzeLogs = true
			req.NoPipeline = true
			req.Transforms = []string{"include-regex:error"}
		})},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Clean = true })},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Dedupe = true })},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Unique = true })},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.K8s = true })},
	}
	for _, opts := range tests {
		if err := validateRootOptions(opts); err == nil {
			t.Fatalf("expected no-pipeline conflict for opts %+v", opts)
		}
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true })}); err != nil {
		t.Fatalf("expected bare --no-pipeline to pass, got %v", err)
	}
}

func TestValidateRootOptionsRejectsInvalidExecuteFlags(t *testing.T) {
	if err := validateRootOptions(rootOptions{Execute: "sandbox"}); err == nil {
		t.Fatal("expected --execute without --shell to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true }), Execute: "weird"}); err == nil {
		t.Fatal("expected invalid execute mode to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.ForceRiskyShell = true }), Execute: "safe"}); err == nil {
		t.Fatal("expected force-risky-shell without host execute to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.SandboxRuntime = "runc" })}); err == nil {
		t.Fatal("expected invalid sandbox runtime to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.SandboxRuntime = "docker" }), Execute: "safe"}); err == nil {
		t.Fatal("expected sandbox options with non-sandbox execute mode to fail")
	}
}

func TestValidateRootOptionsRejectsInvalidProvider(t *testing.T) {
	err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.Provider = "weird"
	})})
	if err == nil {
		t.Fatal("expected invalid provider to fail")
	}
}

func TestValidateRootOptionsAllowsKnownProvider(t *testing.T) {
	err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.Provider = "anthropic"
	})})
	if err != nil {
		t.Fatalf("expected known provider to pass, got %v", err)
	}
}

func TestNormalizeRootOptionsAppliesShellAliases(t *testing.T) {
	opts := rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.Shell = true
			req.SandboxImage = "alpine:3.20"
			req.SandboxNetwork = true
			req.SandboxWrite = true
		}),
		SandboxExec:   true,
		DockerRuntime: true,
	}
	if err := normalizeRootOptions(&opts); err != nil {
		t.Fatalf("normalizeRootOptions failed: %v", err)
	}
	if opts.Execute != "sandbox" {
		t.Fatalf("expected sandbox execute alias, got %q", opts.Execute)
	}
	if opts.Request.SandboxRuntime != "docker" {
		t.Fatalf("expected docker runtime alias, got %q", opts.Request.SandboxRuntime)
	}
	if err := validateRootOptions(opts); err != nil {
		t.Fatalf("validateRootOptions failed after normalization: %v", err)
	}
}

func TestNormalizeRootOptionsRejectsConflictingAliases(t *testing.T) {
	opts := rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true }), Execute: "host", SafeExec: true}
	if err := normalizeRootOptions(&opts); err == nil {
		t.Fatal("expected execute alias conflict")
	}

	opts = rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.SandboxRuntime = "docker" }), PodmanRuntime: true}
	if err := normalizeRootOptions(&opts); err == nil {
		t.Fatal("expected runtime alias conflict")
	}
}

func TestNormalizeRootOptionsAllowsSafeAliasWithoutSandboxOptions(t *testing.T) {
	opts := rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true }), SafeExec: true}
	if err := normalizeRootOptions(&opts); err != nil {
		t.Fatalf("normalizeRootOptions failed: %v", err)
	}
	if opts.Execute != "safe" {
		t.Fatalf("expected safe execute alias, got %q", opts.Execute)
	}
	if err := validateRootOptions(opts); err != nil {
		t.Fatalf("validateRootOptions failed: %v", err)
	}
}

func TestRootCommandExposesAnalyzeLogsAndProfileShorthands(t *testing.T) {
	cmd := NewRootCommand()
	analyze := cmd.Flags().Lookup("analyze-logs")
	if analyze == nil || analyze.Shorthand != "a" {
		t.Fatalf("expected -a shorthand for analyze-logs, got %+v", analyze)
	}
	profile := cmd.Flags().Lookup("profile")
	if profile == nil || profile.Shorthand != "p" {
		t.Fatalf("expected -p shorthand for profile, got %+v", profile)
	}
	if cmd.Flags().Lookup("transform") == nil {
		t.Fatal("expected --transform flag")
	}
	if cmd.Flags().Lookup("clean") == nil || cmd.Flags().Lookup("dedupe") == nil || cmd.Flags().Lookup("unique") == nil || cmd.Flags().Lookup("k8s") == nil {
		t.Fatal("expected log shorthand flags")
	}
	if tail := cmd.Flags().Lookup("tail"); tail == nil {
		t.Fatal("expected --tail alias")
	}
}

func TestRootCommandTailAliasAffectsPromptReduction(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("OPENAI_API_KEY", "test-key")
	withTestStdin(t, "line 1\nline 2\nline 3\n")

	recorder := &handlerFailureRecorder{}
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			recorder.Failf("decode request failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.DefaultModel = "gpt-4o-mini"
	cfg.DefaultProvider = "openai"
	cfg.OpenAIBaseURL = server.URL + "/"
	cfg.Stream = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--tail", "2", "--no-stream", "analyze", "these", "logs"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execute failed: %v", err)
	}
	recorder.Assert(t)

	input, ok := requestBody["input"].([]any)
	if !ok || len(input) == 0 {
		t.Fatalf("unexpected request body: %+v", requestBody)
	}
	firstItem, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first input item: %+v", input[0])
	}
	got, _ := firstItem["content"].(string)
	want := "analyze these logs\n\ninput:\nline 2\nline 3"
	if got != want {
		t.Fatalf("unexpected tail-reduced prompt:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestToAppRequestIncludesPipelineOptions(t *testing.T) {
	req := toAppRequest(rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.AnalyzeLogs = true
			req.Profile = "k8s"
			req.Transforms = []string{"include-regex:error"}
			req.NoPipeline = true
			req.Clean = true
			req.Dedupe = true
			req.K8s = true
		}),
	}, promptSource{instructionText: "analyze", stdinText: "logs"})
	if req.Profile != "k8s" || !req.NoPipeline || !req.Clean || !req.Dedupe || !req.K8s {
		t.Fatalf("unexpected pipeline request wiring: %+v", req)
	}
	if len(req.Transforms) != 1 || req.Transforms[0] != "include-regex:error" {
		t.Fatalf("unexpected transforms: %+v", req.Transforms)
	}
}

func TestLoadLogsStatusReportsCountsAndSize(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	cfg := config.Default()
	cfg.LogToDB = false
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config save failed: %v", err)
	}

	logStore := store.NewLogStore(config.LogsDBPath())
	conversation := "demo"
	if err := logStore.Insert(model.LogEntry{
		Model:        "gpt-4o-mini",
		Provider:     "openai",
		Prompt:       "hello",
		Response:     "world",
		Conversation: &conversation,
	}); err != nil {
		t.Fatalf("log insert failed: %v", err)
	}

	status, err := loadLogsStatus(logStore)
	if err != nil {
		t.Fatalf("loadLogsStatus failed: %v", err)
	}
	if status.Enabled {
		t.Fatalf("expected logging disabled in config")
	}
	if status.Responses != 1 || status.Conversations != 1 {
		t.Fatalf("unexpected log status counts: %+v", status)
	}
	if status.SizeBytes <= 0 {
		t.Fatalf("expected database size, got %+v", status)
	}
}
