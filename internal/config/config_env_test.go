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
	t.Setenv("KNOTICAL_ANTHROPIC_TRANSPORT", "cli")
	t.Setenv("KNOTICAL_GEMINI_TRANSPORT", "cli")
	t.Setenv("KNOTICAL_API_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("KNOTICAL_OPENAI_BASE_URL", "http://localhost:9999")
	t.Setenv("KNOTICAL_ANTHROPIC_BASE_URL", "http://localhost:9998")
	t.Setenv("KNOTICAL_REQUEST_TIMEOUT", "120")
	t.Setenv("KNOTICAL_STREAM", "false")
	t.Setenv("KNOTICAL_PRETTIFY_MARKDOWN", "false")
	t.Setenv("KNOTICAL_LOG_TO_DB", "false")
	t.Setenv("KNOTICAL_SHELL_EXECUTE_MODE", "sandbox")
	t.Setenv("KNOTICAL_SHELL_SANDBOX_RUNTIME", "podman")
	t.Setenv("KNOTICAL_SHELL_SANDBOX_IMAGE", "ubuntu:24.04")
	t.Setenv("KNOTICAL_SHELL_SANDBOX_NETWORK", "true")
	t.Setenv("KNOTICAL_SHELL_SANDBOX_WRITE", "true")
	t.Setenv("KNOTICAL_MAX_INPUT_BYTES", "4096")
	t.Setenv("KNOTICAL_MAX_INPUT_LINES", "200")
	t.Setenv("KNOTICAL_MAX_INPUT_TOKENS", "800")
	t.Setenv("KNOTICAL_INPUT_REDUCTION_MODE", "fail")
	t.Setenv("KNOTICAL_LOG_ANALYSIS_MARKDOWN", "true")
	t.Setenv("KNOTICAL_LOG_ANALYSIS_SCHEMA", "summary, likely_root_cause")
	t.Setenv("KNOTICAL_LOG_ANALYSIS_SYSTEM_PROMPT", "custom log prompt")
	t.Setenv("KNOTICAL_DEFAULT_LOG_PROFILE", "k8s")
	t.Setenv("KNOTICAL_SUMMARIZE_CHUNK_TOKENS", "600")
	t.Setenv("KNOTICAL_SUMMARIZE_CHUNK_OVERLAP_LINES", "7")
	t.Setenv("KNOTICAL_SUMMARIZE_INTERMEDIATE_MODEL", "gpt-4o-mini")
	t.Setenv("KNOTICAL_DEFAULT_HEAD_LINES", "20")
	t.Setenv("KNOTICAL_DEFAULT_TAIL_LINES", "30")
	t.Setenv("KNOTICAL_DEFAULT_SAMPLE_LINES", "40")

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
	if cfg.AnthropicTransport != "cli" {
		t.Fatalf("unexpected anthropic transport: %q", cfg.AnthropicTransport)
	}
	if cfg.GeminiTransport != "cli" {
		t.Fatalf("unexpected gemini transport: %q", cfg.GeminiTransport)
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
	if cfg.ShellExecuteMode != "sandbox" ||
		cfg.ShellSandboxRuntime != "podman" ||
		cfg.ShellSandboxImage != "ubuntu:24.04" ||
		!cfg.ShellSandboxNetwork ||
		!cfg.ShellSandboxWrite {
		t.Fatalf("unexpected shell config overrides: %+v", cfg)
	}
	if cfg.MaxInputBytes != 4096 || cfg.MaxInputLines != 200 || cfg.MaxInputTokens != 800 || cfg.InputReductionMode != "fail" || !cfg.LogAnalysisMarkdown || cfg.LogAnalysisSchema != "summary, likely_root_cause" || cfg.LogAnalysisSystemPrompt != "custom log prompt" || cfg.DefaultLogProfile != "k8s" || cfg.SummarizeChunkTokens != 600 || cfg.SummarizeChunkOverlapLines != 7 || cfg.SummarizeIntermediateModel != "gpt-4o-mini" || cfg.DefaultHeadLines != 20 || cfg.DefaultTailLines != 30 || cfg.DefaultSampleLines != 40 {
		t.Fatalf("unexpected input reduction overrides: %+v", cfg)
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
