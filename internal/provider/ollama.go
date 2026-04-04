package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

type OllamaProvider struct {
	baseURL string
	timeout time.Duration
}

func NewOllamaProvider(baseURL string, timeout time.Duration) OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}
	return OllamaProvider{baseURL: baseURL, timeout: timeout}
}

func (p OllamaProvider) Name() string { return "ollama" }

func (p OllamaProvider) Complete(ctx context.Context, req Request) (model.CompletionResponse, error) {
	client := NewOpenAIProvider("ollama", p.baseURL, p.timeout)
	req.Model = strings.TrimPrefix(req.Model, "ollama/")
	return client.Complete(ctx, req)
}

func (p OllamaProvider) Stream(ctx context.Context, req Request, emit func(model.StreamChunk) error) error {
	client := NewOpenAIProvider("ollama", p.baseURL, p.timeout)
	req.Model = strings.TrimPrefix(req.Model, "ollama/")
	return client.Stream(ctx, req, emit)
}

func (p OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	baseURL := strings.TrimSuffix(strings.TrimSuffix(p.baseURL, "/"), "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: p.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama model list failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(payload.Models))
	for _, item := range payload.Models {
		if item.Name == "" {
			continue
		}
		models = append(models, "ollama/"+item.Name)
	}
	slices.Sort(models)
	return models, nil
}
