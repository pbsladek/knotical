package config

import (
	"os"
	"testing"
)

func TestLoadAppliesEnvOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("KNOTICAL_DEFAULT_MODEL", "claude-sonnet-4-5")
	t.Setenv("KNOTICAL_DEFAULT_PROVIDER", "anthropic")
	t.Setenv("KNOTICAL_API_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("KNOTICAL_OPENAI_BASE_URL", "http://localhost:9999")
	t.Setenv("KNOTICAL_ANTHROPIC_BASE_URL", "http://localhost:9998")
	t.Setenv("KNOTICAL_REQUEST_TIMEOUT", "120")
	t.Setenv("KNOTICAL_STREAM", "false")
	t.Setenv("KNOTICAL_PRETTIFY_MARKDOWN", "false")
	t.Setenv("KNOTICAL_LOG_TO_DB", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DefaultModel != "claude-sonnet-4-5" {
		t.Fatalf("unexpected default model: %q", cfg.DefaultModel)
	}
	if cfg.DefaultProvider != "anthropic" {
		t.Fatalf("unexpected default provider: %q", cfg.DefaultProvider)
	}
	if cfg.APIBaseURL != "http://localhost:11434/v1" {
		t.Fatalf("unexpected legacy API base URL: %q", cfg.APIBaseURL)
	}
	if cfg.OpenAIBaseURL != "http://localhost:9999" {
		t.Fatalf("unexpected OpenAI base URL: %q", cfg.OpenAIBaseURL)
	}
	if cfg.AnthropicBaseURL != "http://localhost:9998" {
		t.Fatalf("unexpected Anthropic base URL: %q", cfg.AnthropicBaseURL)
	}
	if cfg.RequestTimeout != 120 {
		t.Fatalf("unexpected request timeout: %d", cfg.RequestTimeout)
	}
	if cfg.Stream {
		t.Fatalf("expected stream override to be false")
	}
	if cfg.PrettifyMarkdown {
		t.Fatalf("expected prettify_markdown override to be false")
	}
	if cfg.LogToDB {
		t.Fatalf("expected log_to_db override to be false")
	}
}

func TestBaseURLForProviderIsProviderScoped(t *testing.T) {
	cfg := Config{
		APIBaseURL:       "http://legacy-ollama.local/v1",
		OpenAIBaseURL:    "http://openai.local",
		AnthropicBaseURL: "http://anthropic.local",
		GeminiBaseURL:    "http://gemini.local",
	}

	if got := cfg.BaseURLForProvider("openai"); got != "http://openai.local" {
		t.Fatalf("unexpected openai base url: %q", got)
	}
	if got := cfg.BaseURLForProvider("anthropic"); got != "http://anthropic.local" {
		t.Fatalf("unexpected anthropic base url: %q", got)
	}
	if got := cfg.BaseURLForProvider("gemini"); got != "http://gemini.local" {
		t.Fatalf("unexpected gemini base url: %q", got)
	}
	if got := cfg.BaseURLForProvider("ollama"); got != "http://legacy-ollama.local/v1" {
		t.Fatalf("unexpected ollama base url: %q", got)
	}
	if got := cfg.BaseURLForProvider("unknown"); got != "" {
		t.Fatalf("unexpected fallback base url: %q", got)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	original := Default()
	original.DefaultModel = "gemini-2.5-flash"
	original.DefaultProvider = "gemini"
	original.RequestTimeout = 90
	original.Stream = false
	original.LogToDB = false
	original.Temperature = 0.4
	original.TopP = 0.8

	if err := Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.DefaultModel != original.DefaultModel ||
		loaded.DefaultProvider != original.DefaultProvider ||
		loaded.RequestTimeout != original.RequestTimeout ||
		loaded.Stream != original.Stream ||
		loaded.LogToDB != original.LogToDB ||
		loaded.Temperature != original.Temperature ||
		loaded.TopP != original.TopP {
		t.Fatalf("unexpected config round trip: %+v", loaded)
	}
}

func TestSaveUsesSecurePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	if err := Save(Default()); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	info, err := os.Stat(ConfigFilePath())
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected secure config mode, got %o", got)
	}
}

func TestParseBool(t *testing.T) {
	if !parseBool("true", false) {
		t.Fatalf("expected true parse")
	}
	if parseBool("off", true) {
		t.Fatalf("expected false parse")
	}
	if !parseBool("unknown", true) {
		t.Fatalf("expected fallback to true")
	}
}
