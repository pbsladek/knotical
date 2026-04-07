package config

import "testing"

func TestDefaultIncludesTransportAndShellDefaults(t *testing.T) {
	cfg := Default()

	if cfg.OpenAITransport != "api" {
		t.Fatalf("unexpected openai transport default: %q", cfg.OpenAITransport)
	}
	if cfg.AnthropicTransport != "api" {
		t.Fatalf("unexpected anthropic transport default: %q", cfg.AnthropicTransport)
	}
	if cfg.GeminiTransport != "api" {
		t.Fatalf("unexpected gemini transport default: %q", cfg.GeminiTransport)
	}
	if cfg.DefaultLogProfile != "" {
		t.Fatalf("unexpected default log profile default: %q", cfg.DefaultLogProfile)
	}
	if cfg.ShellSandboxImage != "docker.io/library/ubuntu:24.04" {
		t.Fatalf("unexpected shell sandbox image default: %q", cfg.ShellSandboxImage)
	}
	if cfg.ClaudeCLICommand != "claude" || len(cfg.ClaudeCLIArgs) == 0 {
		t.Fatalf("unexpected claude cli defaults: %+v", cfg)
	}
	if cfg.CodexCLICommand != "codex" || len(cfg.CodexCLIArgs) == 0 {
		t.Fatalf("unexpected codex cli defaults: %+v", cfg)
	}
	if cfg.GeminiCLICommand != "gemini" || len(cfg.GeminiCLIArgs) == 0 {
		t.Fatalf("unexpected gemini cli defaults: %+v", cfg)
	}
}
