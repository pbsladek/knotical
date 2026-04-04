package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaListModels(t *testing.T) {
	recorder := &handlerFailureRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			recorder.Failf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/api/tags" {
			recorder.Failf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "models":[
		    {"name":"qwen2.5-coder"},
		    {"name":"llama3.2"}
		  ]
		}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL+"/v1", 0)
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	recorder.Assert(t)
	if got := strings.Join(models, ","); got != "ollama/llama3.2,ollama/qwen2.5-coder" {
		t.Fatalf("unexpected models: %v", models)
	}
}
