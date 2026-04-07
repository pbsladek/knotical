package config

import "testing"

func TestBaseURLForProviderIsProviderScoped(t *testing.T) {
	cfg := Config{
		ProviderTransportConfig: ProviderTransportConfig{
			APIBaseURL:       "http://legacy-ollama.local/v1",
			OpenAIBaseURL:    "http://openai.local",
			AnthropicBaseURL: "http://anthropic.local",
			GeminiBaseURL:    "http://gemini.local",
		},
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

func TestTransportForProviderIsProviderScoped(t *testing.T) {
	cfg := Config{
		ProviderTransportConfig: ProviderTransportConfig{
			OpenAITransport:    "cli",
			AnthropicTransport: "api",
			GeminiTransport:    "cli",
		},
	}

	if got := cfg.TransportForProvider("openai"); got != "cli" {
		t.Fatalf("unexpected openai transport: %q", got)
	}
	if got := cfg.TransportForProvider("anthropic"); got != "api" {
		t.Fatalf("unexpected anthropic transport: %q", got)
	}
	if got := cfg.TransportForProvider("gemini"); got != "cli" {
		t.Fatalf("unexpected gemini transport: %q", got)
	}
	if got := cfg.TransportForProvider("ollama"); got != "api" {
		t.Fatalf("unexpected ollama transport: %q", got)
	}
}

func TestTransportForProviderNormalizesUnknownValuesToAPI(t *testing.T) {
	cfg := Config{
		ProviderTransportConfig: ProviderTransportConfig{
			OpenAITransport:    "CLI",
			AnthropicTransport: "weird",
			GeminiTransport:    "  cli  ",
		},
	}

	if got := cfg.TransportForProvider("openai"); got != "cli" {
		t.Fatalf("expected normalized cli transport, got %q", got)
	}
	if got := cfg.TransportForProvider("anthropic"); got != "api" {
		t.Fatalf("expected unknown transport to fall back to api, got %q", got)
	}
	if got := cfg.TransportForProvider("gemini"); got != "cli" {
		t.Fatalf("expected trimmed cli transport, got %q", got)
	}
}

func TestCLIConfigForProvider(t *testing.T) {
	cfg := Default()

	claude := cfg.CLIConfigForProvider("anthropic")
	if claude.Command != "claude" || claude.SystemFlag != "--system-prompt" || claude.SchemaFlag != "--json-schema" {
		t.Fatalf("unexpected claude cli config: %+v", claude)
	}

	codex := cfg.CLIConfigForProvider("openai")
	if codex.Command != "codex" || len(codex.Args) != 1 || codex.Args[0] != "exec" {
		t.Fatalf("unexpected codex cli config: %+v", codex)
	}
	if codex.ModelFlag != "" || codex.SystemFlag != "" || codex.SchemaFlag != "" {
		t.Fatalf("unexpected codex cli flag defaults: %+v", codex)
	}

	gemini := cfg.CLIConfigForProvider("gemini")
	if gemini.Command != "gemini" || gemini.ModelFlag != "--model" {
		t.Fatalf("unexpected gemini cli config: %+v", gemini)
	}
	if len(gemini.Args) != 1 || gemini.Args[0] != "-p" || gemini.SystemFlag != "" || gemini.SchemaFlag != "" {
		t.Fatalf("unexpected gemini cli defaults: %+v", gemini)
	}
}

func TestConfigViews(t *testing.T) {
	cfg := Default()
	cfg.DefaultModel = "claude-sonnet-4-5"
	cfg.DefaultProvider = "anthropic"
	cfg.RequestTimeout = 90
	cfg.ShellExecuteMode = "sandbox"
	cfg.ShellSandboxRuntime = "podman"
	cfg.ShellSandboxImage = "ubuntu:24.04"
	cfg.ShellSandboxNetwork = true
	cfg.ShellSandboxWrite = true
	cfg.MaxInputBytes = 4096
	cfg.MaxInputLines = 200
	cfg.MaxInputTokens = 800
	cfg.InputReductionMode = "fail"
	cfg.DefaultHeadLines = 10
	cfg.DefaultTailLines = 20
	cfg.DefaultSampleLines = 30
	cfg.LogAnalysisMarkdown = true
	cfg.LogAnalysisSchema = "summary"
	cfg.LogAnalysisSystemPrompt = "custom"
	cfg.DefaultLogProfile = "k8s"
	cfg.SummarizeChunkTokens = 600
	cfg.SummarizeChunkOverlapLines = 7
	cfg.SummarizeIntermediateModel = "gpt-4o-mini"

	providerCfg := cfg.ProviderSettings()
	if providerCfg.DefaultModel != "claude-sonnet-4-5" || providerCfg.DefaultProvider != "anthropic" || providerCfg.RequestTimeout.Seconds() != 90 {
		t.Fatalf("unexpected provider settings: %+v", providerCfg)
	}
	runtimeCfg := cfg.ProviderRuntime("anthropic")
	if runtimeCfg.Transport != "api" || runtimeCfg.BaseURL != "" || runtimeCfg.CLI.Command != "claude" {
		t.Fatalf("unexpected provider runtime: %+v", runtimeCfg)
	}
	if !runtimeCfg.Capabilities.NativeSchema || !runtimeCfg.Capabilities.ModelListing {
		t.Fatalf("unexpected provider capabilities: %+v", runtimeCfg.Capabilities)
	}

	shellCfg := cfg.ShellSettings()
	if shellCfg.ExecuteMode != "sandbox" || shellCfg.Runtime != "podman" || shellCfg.Image != "ubuntu:24.04" || !shellCfg.Network || !shellCfg.Write {
		t.Fatalf("unexpected shell settings: %+v", shellCfg)
	}

	ingestCfg := cfg.IngestSettings()
	if ingestCfg.MaxInputBytes != 4096 || ingestCfg.MaxInputLines != 200 || ingestCfg.MaxInputTokens != 800 || ingestCfg.InputReductionMode != "fail" || ingestCfg.DefaultHeadLines != 10 || ingestCfg.DefaultTailLines != 20 || ingestCfg.DefaultSampleLines != 30 {
		t.Fatalf("unexpected ingest settings: %+v", ingestCfg)
	}

	logCfg := cfg.LogAnalysisSettings()
	if !logCfg.Markdown || logCfg.Schema != "summary" || logCfg.SystemPrompt != "custom" || logCfg.DefaultProfile != "k8s" {
		t.Fatalf("unexpected log analysis settings: %+v", logCfg)
	}

	summarizeCfg := cfg.SummarizeSettings()
	if summarizeCfg.ChunkTokens != 600 || summarizeCfg.ChunkOverlapLines != 7 || summarizeCfg.IntermediateModel != "gpt-4o-mini" {
		t.Fatalf("unexpected summarize settings: %+v", summarizeCfg)
	}

	endpoint := cfg.ProviderEndpoint("anthropic")
	if endpoint.Transport != "api" || endpoint.BaseURL != "" || endpoint.CLI.Command != "claude" {
		t.Fatalf("unexpected provider endpoint: %+v", endpoint)
	}
}
