package config

import (
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/ingest"
)

func (cfg Config) Validate() error {
	if err := cfg.validateProviderConfig(); err != nil {
		return err
	}
	if err := cfg.validateNumericConfig(); err != nil {
		return err
	}
	if err := cfg.validateShellConfig(); err != nil {
		return err
	}
	return cfg.validateIngestConfig()
}

func (cfg Config) validateProviderConfig() error {
	for _, name := range []string{"openai", "anthropic", "gemini"} {
		switch cfg.TransportForProvider(name) {
		case "api", "cli":
		default:
			return fmt.Errorf("invalid %s transport", name)
		}
	}
	if cfg.DefaultProvider == "" {
		return nil
	}
	switch cfg.DefaultProvider {
	case "openai", "anthropic", "gemini", "ollama":
		return nil
	default:
		return fmt.Errorf("default_provider must be openai, anthropic, gemini, or ollama")
	}
}

func (cfg Config) validateNumericConfig() error {
	if err := validateCoreNumericConfig(cfg); err != nil {
		return err
	}
	if err := validateIngestNumericConfig(cfg); err != nil {
		return err
	}
	return validateSummarizeNumericConfig(cfg)
}

func validateCoreNumericConfig(cfg Config) error {
	if cfg.RequestTimeout < 0 || cfg.ChatCacheLength < 0 {
		return fmt.Errorf("request_timeout and chat_cache_length must be >= 0")
	}
	return nil
}

func validateIngestNumericConfig(cfg Config) error {
	if cfg.MaxInputBytes < 0 || cfg.MaxInputLines < 0 || cfg.MaxInputTokens < 0 ||
		cfg.DefaultHeadLines < 0 || cfg.DefaultTailLines < 0 || cfg.DefaultSampleLines < 0 {
		return fmt.Errorf("ingest limits must be >= 0")
	}
	return nil
}

func validateSummarizeNumericConfig(cfg Config) error {
	if cfg.SummarizeChunkTokens < 0 || cfg.SummarizeChunkOverlapLines < 0 {
		return fmt.Errorf("summarize limits must be >= 0")
	}
	return nil
}

func (cfg Config) validateShellConfig() error {
	switch strings.ToLower(strings.TrimSpace(cfg.ShellExecuteMode)) {
	case "", "host", "safe", "sandbox":
	default:
		return fmt.Errorf("shell_execute_mode must be host, safe, sandbox, or empty")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ShellSandboxRuntime)) {
	case "", "docker", "podman":
	default:
		return fmt.Errorf("shell_sandbox_runtime must be docker, podman, or empty")
	}
	return nil
}

func (cfg Config) validateIngestConfig() error {
	switch strings.ToLower(strings.TrimSpace(cfg.InputReductionMode)) {
	case "", "off", "truncate", "fail", "summarize":
	default:
		return fmt.Errorf("input_reduction_mode must be off, truncate, fail, summarize, or empty")
	}
	if cfg.DefaultLogProfile != "" {
		if _, ok := ingest.LookupProfile(cfg.DefaultLogProfile); !ok {
			return fmt.Errorf("unknown default_log_profile %q", cfg.DefaultLogProfile)
		}
	}
	return nil
}
