package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/store"
)

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
