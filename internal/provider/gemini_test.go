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

func TestGeminiComplete(t *testing.T) {
	var requestBody map[string]any
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
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
		  "candidates":[
		    {"content":{"parts":[{"text":"Hello from Gemini!"}]}}
		  ],
		  "usageMetadata":{
		    "promptTokenCount":12,
		    "candidatesTokenCount":6,
		    "totalTokenCount":18
		  }
		}`))
	}))
	defer server.Close()

	provider, err := NewGeminiProvider("test-key", server.URL+"/", 0)
	if err != nil {
		t.Fatalf("NewGeminiProvider failed: %v", err)
	}
	temperature := 0.4
	topP := 0.9
	response, err := provider.Complete(context.Background(), Request{
		Model:       "gemini-2.5-flash",
		System:      "Be terse.",
		Messages:    []model.Message{{Role: model.RoleUser, Content: "say hello"}, {Role: model.RoleAssistant, Content: "hello"}},
		Temperature: &temperature,
		TopP:        &topP,
		MaxTokens:   321,
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
	if response.Content != "Hello from Gemini!" {
		t.Fatalf("unexpected response content: %q", response.Content)
	}
	if response.Usage == nil || response.Usage.PromptTokens != 12 || response.Usage.CompletionTokens != 6 || response.Usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %+v", response.Usage)
	}
	configBody := requestBody["generationConfig"].(map[string]any)
	if configBody["temperature"] != temperature {
		t.Fatalf("unexpected temperature: %#v", configBody["temperature"])
	}
	if configBody["topP"] != topP {
		t.Fatalf("unexpected topP: %#v", configBody["topP"])
	}
	if configBody["maxOutputTokens"] != float64(321) {
		t.Fatalf("unexpected maxOutputTokens: %#v", configBody["maxOutputTokens"])
	}
	if configBody["responseMimeType"] != "application/json" {
		t.Fatalf("unexpected responseMimeType: %#v", configBody["responseMimeType"])
	}
	if _, ok := configBody["responseJsonSchema"].(map[string]any); !ok {
		t.Fatalf("expected responseJsonSchema in request, got %#v", configBody["responseJsonSchema"])
	}
	systemInstruction := requestBody["systemInstruction"].(map[string]any)
	parts := systemInstruction["parts"].([]any)
	if parts[0].(map[string]any)["text"] != "Be terse." {
		t.Fatalf("unexpected system instruction: %#v", systemInstruction)
	}
	contents := requestBody["contents"].([]any)
	if len(contents) != 2 {
		t.Fatalf("expected 2 message contents, got %#v", contents)
	}
}

func TestGeminiStreamUsesStreamingEndpoint(t *testing.T) {
	paths := []string{}
	var usage *model.TokenUsage
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if !strings.Contains(r.URL.Path, ":streamGenerateContent") {
			recorder.Failf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`,
			"",
			`data: {"candidates":[{"content":{"parts":[{"text":" world"}]}}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":2,"totalTokenCount":10}}`,
			"",
			"",
		}, "\n"))
	}))
	defer server.Close()

	provider, err := newGeminiProvider("test-key", server.URL+"/", server.Client(), 0)
	if err != nil {
		t.Fatalf("newGeminiProvider failed: %v", err)
	}
	var collected strings.Builder
	err = provider.Stream(context.Background(), Request{
		Model:    "gemini-2.5-flash",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
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
	if collected.String() != "Hello world" {
		t.Fatalf("unexpected collected output: %q", collected.String())
	}
	if usage == nil || usage.PromptTokens != 8 || usage.CompletionTokens != 2 || usage.TotalTokens != 10 {
		t.Fatalf("unexpected stream usage: %+v", usage)
	}
	if len(paths) == 0 {
		t.Fatalf("expected streaming request path")
	}
}
