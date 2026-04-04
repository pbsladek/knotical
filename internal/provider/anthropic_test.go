package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/model"
)

func TestAnthropicComplete(t *testing.T) {
	var requestBody map[string]any
	recorder := &handlerFailureRecorder{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			recorder.Failf("failed to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(body, &requestBody); err != nil {
			recorder.Failf("failed to unmarshal request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id":"msg_123",
		  "type":"message",
		  "role":"assistant",
		  "model":"claude-sonnet-4-5",
		  "content":[{"type":"text","text":"Hello from Claude!"}],
		  "stop_reason":"end_turn",
		  "usage":{"input_tokens":10,"output_tokens":5}
		}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", server.URL+"/", 0)
	temperature := 0.3
	topP := 0.7
	response, err := provider.Complete(context.Background(), Request{
		Model:       "claude-sonnet-4-5",
		System:      "Be terse.",
		Messages:    []model.Message{{Role: model.RoleUser, Content: "say hello"}},
		Temperature: &temperature,
		TopP:        &topP,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	recorder.Assert(t)
	if response.Content != "Hello from Claude!" {
		t.Fatalf("unexpected response content: %q", response.Content)
	}
	if response.Usage == nil || response.Usage.PromptTokens != 10 || response.Usage.CompletionTokens != 5 {
		t.Fatalf("unexpected usage: %+v", response.Usage)
	}

	systemBlocks, ok := requestBody["system"].([]any)
	if !ok || len(systemBlocks) == 0 {
		t.Fatalf("expected system blocks in request: %#v", requestBody["system"])
	}
	firstSystem := systemBlocks[0].(map[string]any)
	if firstSystem["text"] != "Be terse." {
		t.Fatalf("unexpected system prompt: %#v", firstSystem)
	}
	if requestBody["temperature"] != temperature {
		t.Fatalf("expected temperature in request, got %#v", requestBody["temperature"])
	}
	if requestBody["top_p"] != topP {
		t.Fatalf("expected top_p in request, got %#v", requestBody["top_p"])
	}
}

func TestAnthropicListModels(t *testing.T) {
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			recorder.Failf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/v1/models" {
			recorder.Failf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "data":[
		    {"id":"claude-opus-4-1","created_at":"2024-01-01T00:00:00Z","display_name":"Claude Opus 4.1","type":"model"},
		    {"id":"claude-sonnet-4-5","created_at":"2024-01-02T00:00:00Z","display_name":"Claude Sonnet 4.5","type":"model"}
		  ],
		  "has_more":false,
		  "first_id":"claude-opus-4-1",
		  "last_id":"claude-sonnet-4-5"
		}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", server.URL+"/", 0)
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	recorder.Assert(t)
	if got := strings.Join(models, ","); got != "claude-opus-4-1,claude-sonnet-4-5" {
		t.Fatalf("unexpected models: %v", models)
	}
}

func TestAnthropicStreamIncludesSystemPrompt(t *testing.T) {
	var requestBody map[string]any
	var usage *model.TokenUsage
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			recorder.Failf("failed to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(body, &requestBody); err != nil {
			recorder.Failf("failed to unmarshal request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			"",
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":11,"output_tokens":4,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"server_tool_use":{"web_search_requests":0}}}`,
			"",
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			"",
			"",
		}, "\n"))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", server.URL+"/", 0)
	err := provider.Stream(context.Background(), Request{
		Model:    "claude-sonnet-4-5",
		System:   "Be terse.",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	}, func(chunk model.StreamChunk) error {
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	recorder.Assert(t)
	systemBlocks := requestBody["system"].([]any)
	if systemBlocks[0].(map[string]any)["text"] != "Be terse." {
		t.Fatalf("unexpected system prompt: %#v", requestBody["system"])
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 4 || usage.TotalTokens != 15 {
		t.Fatalf("unexpected stream usage: %+v", usage)
	}
}
