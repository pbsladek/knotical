package config

import (
	"os"
	"strconv"
	"strings"
)

func applyEnvOverrides(cfg *Config) {
	applyProviderEnvOverrides(cfg)
	applyBaseURLEnvOverrides(cfg)
	applyBehaviorEnvOverrides(cfg)
	applyShellEnvOverrides(cfg)
	applyIngestEnvOverrides(cfg)
	applyLogAnalysisEnvOverrides(cfg)
	applySummarizeEnvOverrides(cfg)
}

func applyProviderEnvOverrides(cfg *Config) {
	applyStringEnv("KNOTICAL_DEFAULT_MODEL", &cfg.DefaultModel)
	applyStringEnv("KNOTICAL_DEFAULT_PROVIDER", &cfg.DefaultProvider)
	applyStringEnv("KNOTICAL_OPENAI_TRANSPORT", &cfg.OpenAITransport)
	applyStringEnv("KNOTICAL_ANTHROPIC_TRANSPORT", &cfg.AnthropicTransport)
	applyStringEnv("KNOTICAL_GEMINI_TRANSPORT", &cfg.GeminiTransport)
}

func applyBaseURLEnvOverrides(cfg *Config) {
	applyStringEnv("KNOTICAL_API_BASE_URL", &cfg.APIBaseURL)
	applyStringEnv("KNOTICAL_OPENAI_BASE_URL", &cfg.OpenAIBaseURL)
	applyStringEnv("KNOTICAL_ANTHROPIC_BASE_URL", &cfg.AnthropicBaseURL)
	applyStringEnv("KNOTICAL_GEMINI_BASE_URL", &cfg.GeminiBaseURL)
	applyStringEnv("KNOTICAL_OLLAMA_BASE_URL", &cfg.OllamaBaseURL)
}

func applyBehaviorEnvOverrides(cfg *Config) {
	applyIntEnv("KNOTICAL_REQUEST_TIMEOUT", &cfg.RequestTimeout)
	applyBoolEnv("KNOTICAL_STREAM", &cfg.Stream)
	applyBoolEnv("KNOTICAL_PRETTIFY_MARKDOWN", &cfg.PrettifyMarkdown)
	applyBoolEnv("KNOTICAL_LOG_TO_DB", &cfg.LogToDB)
}

func applyShellEnvOverrides(cfg *Config) {
	applyStringEnv("KNOTICAL_SHELL_EXECUTE_MODE", &cfg.ShellExecuteMode)
	applyStringEnv("KNOTICAL_SHELL_SANDBOX_RUNTIME", &cfg.ShellSandboxRuntime)
	applyStringEnv("KNOTICAL_SHELL_SANDBOX_IMAGE", &cfg.ShellSandboxImage)
	applyBoolEnv("KNOTICAL_SHELL_SANDBOX_NETWORK", &cfg.ShellSandboxNetwork)
	applyBoolEnv("KNOTICAL_SHELL_SANDBOX_WRITE", &cfg.ShellSandboxWrite)
}

func applyIngestEnvOverrides(cfg *Config) {
	applyIntEnv("KNOTICAL_MAX_INPUT_BYTES", &cfg.MaxInputBytes)
	applyIntEnv("KNOTICAL_MAX_INPUT_LINES", &cfg.MaxInputLines)
	applyIntEnv("KNOTICAL_MAX_INPUT_TOKENS", &cfg.MaxInputTokens)
	applyStringEnv("KNOTICAL_INPUT_REDUCTION_MODE", &cfg.InputReductionMode)
	applyIntEnv("KNOTICAL_DEFAULT_HEAD_LINES", &cfg.DefaultHeadLines)
	applyIntEnv("KNOTICAL_DEFAULT_TAIL_LINES", &cfg.DefaultTailLines)
	applyIntEnv("KNOTICAL_DEFAULT_SAMPLE_LINES", &cfg.DefaultSampleLines)
}

func applyLogAnalysisEnvOverrides(cfg *Config) {
	applyBoolEnv("KNOTICAL_LOG_ANALYSIS_MARKDOWN", &cfg.LogAnalysisMarkdown)
	applyStringEnv("KNOTICAL_LOG_ANALYSIS_SCHEMA", &cfg.LogAnalysisSchema)
	applyStringEnv("KNOTICAL_LOG_ANALYSIS_SYSTEM_PROMPT", &cfg.LogAnalysisSystemPrompt)
	applyStringEnv("KNOTICAL_DEFAULT_LOG_PROFILE", &cfg.DefaultLogProfile)
}

func applySummarizeEnvOverrides(cfg *Config) {
	applyIntEnv("KNOTICAL_SUMMARIZE_CHUNK_TOKENS", &cfg.SummarizeChunkTokens)
	applyIntEnv("KNOTICAL_SUMMARIZE_CHUNK_OVERLAP_LINES", &cfg.SummarizeChunkOverlapLines)
	applyStringEnv("KNOTICAL_SUMMARIZE_INTERMEDIATE_MODEL", &cfg.SummarizeIntermediateModel)
}

func applyStringEnv(name string, target *string) {
	if value := os.Getenv(name); value != "" {
		*target = value
	}
}

func applyIntEnv(name string, target *int) {
	if value := os.Getenv(name); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			*target = parsed
		}
	}
}

func applyBoolEnv(name string, target *bool) {
	if value := os.Getenv(name); value != "" {
		*target = parseBool(value, *target)
	}
}

func parseBool(input string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
