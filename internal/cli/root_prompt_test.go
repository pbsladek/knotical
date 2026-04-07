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
