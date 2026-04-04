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

func TestOpenAIComplete(t *testing.T) {
	var requestBody map[string]any
	recorder := &handlerFailureRecorder{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			recorder.Failf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/responses" {
			recorder.Failf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

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
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello from mock!"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL+"/", 0)
	response, err := provider.Complete(context.Background(), Request{
		Model:    "gpt-4o-mini",
		System:   "You are a pirate.",
		Messages: []model.Message{{Role: model.RoleUser, Content: "say hello"}},
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	recorder.Assert(t)

	if response.Content != "Hello from mock!" {
		t.Fatalf("unexpected response content: %q", response.Content)
	}
	if response.Usage == nil || response.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", response.Usage)
	}
	if requestBody["instructions"] != "You are a pirate." {
		t.Fatalf("expected instructions, got %#v", requestBody["instructions"])
	}

	inputItems, ok := requestBody["input"].([]any)
	if !ok || len(inputItems) != 1 {
		t.Fatalf("unexpected input payload: %#v", requestBody["input"])
	}
	firstInput := inputItems[0].(map[string]any)
	if firstInput["role"] != "user" || firstInput["content"] != "say hello" {
		t.Fatalf("expected user input item, got %#v", firstInput)
	}
}

func TestOpenAIStream(t *testing.T) {
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
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello","logprobs":[],"sequence_number":1}`,
			"",
			`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":" world","logprobs":[],"sequence_number":2}`,
			"",
			`data: {"type":"response.completed","sequence_number":3,"response":{"id":"resp_123","object":"response","created_at":1700000000,"status":"completed","model":"gpt-4o-mini","output":[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello world"}]}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`,
			"",
			"",
		}, "\n"))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL+"/", 0)
	var collected strings.Builder
	var usage *model.TokenUsage
	temperature := 0.6
	topP := 0.8
	err := provider.Stream(context.Background(), Request{
		Model:       "gpt-4o-mini",
		System:      "be terse",
		Messages:    []model.Message{{Role: model.RoleUser, Content: "hi"}},
		Temperature: &temperature,
		TopP:        &topP,
		MaxTokens:   123,
		Stream:      true,
	}, func(chunk model.StreamChunk) error {
		collected.WriteString(chunk.Delta)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	recorder.Assert(t)

	if got := collected.String(); got != "Hello world" {
		t.Fatalf("unexpected collected output: %q", got)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 2 || usage.TotalTokens != 12 {
		t.Fatalf("unexpected stream usage: %+v", usage)
	}
	if requestBody["temperature"] != temperature {
		t.Fatalf("expected temperature in stream request, got %#v", requestBody["temperature"])
	}
	if requestBody["top_p"] != topP {
		t.Fatalf("expected top_p in stream request, got %#v", requestBody["top_p"])
	}
	if requestBody["max_output_tokens"] != float64(123) {
		t.Fatalf("expected max_output_tokens in stream request, got %#v", requestBody["max_output_tokens"])
	}
}

func TestOpenAICompleteUsesNativeSchema(t *testing.T) {
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
		  "id":"resp_test",
		  "object":"response",
		  "created_at":1700000000,
		  "status":"completed",
		  "model":"gpt-4o-mini",
		  "output":[{"id":"msg_123","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"{\"name\":\"alice\"}"}]}],
		  "usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL+"/", 0)
	_, err := provider.Complete(context.Background(), Request{
		Model:    "gpt-4o-mini",
		Messages: []model.Message{{Role: model.RoleUser, Content: "make a user"}},
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	recorder.Assert(t)

	textConfig, ok := requestBody["text"].(map[string]any)
	if !ok {
		t.Fatalf("expected text config, got %#v", requestBody["text"])
	}
	format := textConfig["format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Fatalf("expected json_schema format, got %#v", format)
	}
}

func TestOpenAIListModels(t *testing.T) {
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			recorder.Failf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/models" {
			recorder.Failf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "object":"list",
		  "data":[
		    {"id":"gpt-4o","object":"model","created":1,"owned_by":"openai"},
		    {"id":"gpt-4o-mini","object":"model","created":1,"owned_by":"openai"}
		  ]
		}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL+"/", 0)
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	recorder.Assert(t)
	if got := strings.Join(models, ","); got != "gpt-4o,gpt-4o-mini" {
		t.Fatalf("unexpected models: %v", models)
	}
}
