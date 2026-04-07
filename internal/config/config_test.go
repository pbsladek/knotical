package config

import (
	"os"
	"testing"
)

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

func TestValidateRejectsInvalidConfigValues(t *testing.T) {
	tests := []Config{
		func() Config {
			cfg := Default()
			cfg.DefaultProvider = "weird"
			return cfg
		}(),
		func() Config {
			cfg := Default()
			cfg.ShellExecuteMode = "boom"
			return cfg
		}(),
		func() Config {
			cfg := Default()
			cfg.ShellSandboxRuntime = "runc"
			return cfg
		}(),
		func() Config {
			cfg := Default()
			cfg.InputReductionMode = "mystery"
			return cfg
		}(),
		func() Config {
			cfg := Default()
			cfg.DefaultLogProfile = "unknown"
			return cfg
		}(),
	}

	for _, cfg := range tests {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected config validation failure for %+v", cfg)
		}
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

func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	original := Default()
	original.DefaultModel = "gemini-2.5-flash"
	original.DefaultProvider = "gemini"
	original.GeminiTransport = "cli"
	original.RequestTimeout = 90
	original.Stream = false
	original.LogToDB = false
	original.Temperature = 0.4
	original.TopP = 0.8
	original.ShellExecuteMode = "sandbox"
	original.ShellSandboxRuntime = "docker"
	original.ShellSandboxImage = "ubuntu:24.04"
	original.ShellSandboxNetwork = true
	original.ShellSandboxWrite = true
	original.MaxInputBytes = 4096
	original.MaxInputLines = 200
	original.MaxInputTokens = 800
	original.InputReductionMode = "fail"
	original.LogAnalysisMarkdown = true
	original.LogAnalysisSchema = "summary, likely_root_cause"
	original.LogAnalysisSystemPrompt = "custom log prompt"
	original.DefaultLogProfile = "k8s"
	original.SummarizeChunkTokens = 600
	original.SummarizeChunkOverlapLines = 7
	original.SummarizeIntermediateModel = "gpt-4o-mini"
	original.DefaultHeadLines = 20
	original.DefaultTailLines = 30
	original.DefaultSampleLines = 40

	if err := Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.DefaultModel != original.DefaultModel ||
		loaded.DefaultProvider != original.DefaultProvider ||
		loaded.GeminiTransport != original.GeminiTransport ||
		loaded.RequestTimeout != original.RequestTimeout ||
		loaded.Stream != original.Stream ||
		loaded.LogToDB != original.LogToDB ||
		loaded.Temperature != original.Temperature ||
		loaded.TopP != original.TopP ||
		loaded.ShellExecuteMode != original.ShellExecuteMode ||
		loaded.ShellSandboxRuntime != original.ShellSandboxRuntime ||
		loaded.ShellSandboxImage != original.ShellSandboxImage ||
		loaded.ShellSandboxNetwork != original.ShellSandboxNetwork ||
		loaded.ShellSandboxWrite != original.ShellSandboxWrite ||
		loaded.MaxInputBytes != original.MaxInputBytes ||
		loaded.MaxInputLines != original.MaxInputLines ||
		loaded.MaxInputTokens != original.MaxInputTokens ||
		loaded.InputReductionMode != original.InputReductionMode ||
		loaded.LogAnalysisMarkdown != original.LogAnalysisMarkdown ||
		loaded.LogAnalysisSchema != original.LogAnalysisSchema ||
		loaded.LogAnalysisSystemPrompt != original.LogAnalysisSystemPrompt ||
		loaded.DefaultLogProfile != original.DefaultLogProfile ||
		loaded.SummarizeChunkTokens != original.SummarizeChunkTokens ||
		loaded.SummarizeChunkOverlapLines != original.SummarizeChunkOverlapLines ||
		loaded.SummarizeIntermediateModel != original.SummarizeIntermediateModel ||
		loaded.DefaultHeadLines != original.DefaultHeadLines ||
		loaded.DefaultTailLines != original.DefaultTailLines ||
		loaded.DefaultSampleLines != original.DefaultSampleLines {
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
