package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pbsladek/knotical/internal/ingest"
	"github.com/pbsladek/knotical/internal/provider"
)

type CoreConfig struct {
	DefaultModel           string  `toml:"default_model"`
	DefaultProvider        string  `toml:"default_provider"`
	RequestTimeout         int     `toml:"request_timeout"`
	ChatCacheLength        int     `toml:"chat_cache_length"`
	Stream                 bool    `toml:"stream"`
	PrettifyMarkdown       bool    `toml:"prettify_markdown"`
	DefaultColor           string  `toml:"default_color"`
	DefaultExecuteShellCmd bool    `toml:"default_execute_shell_cmd"`
	LogToDB                bool    `toml:"log_to_db"`
	Temperature            float64 `toml:"temperature"`
	TopP                   float64 `toml:"top_p"`
}

type ProviderTransportConfig struct {
	OpenAITransport    string `toml:"openai_transport"`
	AnthropicTransport string `toml:"anthropic_transport"`
	GeminiTransport    string `toml:"gemini_transport"`
	APIBaseURL         string `toml:"api_base_url"`
	OpenAIBaseURL      string `toml:"openai_base_url"`
	AnthropicBaseURL   string `toml:"anthropic_base_url"`
	GeminiBaseURL      string `toml:"gemini_base_url"`
	OllamaBaseURL      string `toml:"ollama_base_url"`
}

type ShellConfig struct {
	ShellExecuteMode    string `toml:"shell_execute_mode"`
	ShellSandboxRuntime string `toml:"shell_sandbox_runtime"`
	ShellSandboxImage   string `toml:"shell_sandbox_image"`
	ShellSandboxNetwork bool   `toml:"shell_sandbox_network"`
	ShellSandboxWrite   bool   `toml:"shell_sandbox_write"`
}

type IngestConfig struct {
	MaxInputBytes      int    `toml:"max_input_bytes"`
	MaxInputLines      int    `toml:"max_input_lines"`
	MaxInputTokens     int    `toml:"max_input_tokens"`
	InputReductionMode string `toml:"input_reduction_mode"`
	DefaultHeadLines   int    `toml:"default_head_lines"`
	DefaultTailLines   int    `toml:"default_tail_lines"`
	DefaultSampleLines int    `toml:"default_sample_lines"`
	DefaultLogProfile  string `toml:"default_log_profile"`
}

type LogAnalysisConfigSection struct {
	LogAnalysisMarkdown     bool   `toml:"log_analysis_markdown"`
	LogAnalysisSchema       string `toml:"log_analysis_schema"`
	LogAnalysisSystemPrompt string `toml:"log_analysis_system_prompt"`
}

type SummarizeConfigSection struct {
	SummarizeChunkTokens       int    `toml:"summarize_chunk_tokens"`
	SummarizeChunkOverlapLines int    `toml:"summarize_chunk_overlap_lines"`
	SummarizeIntermediateModel string `toml:"summarize_intermediate_model"`
}

type ClaudeCLIConfigSection struct {
	ClaudeCLICommand    string   `toml:"claude_cli_command"`
	ClaudeCLIArgs       []string `toml:"claude_cli_args"`
	ClaudeCLIModelFlag  string   `toml:"claude_cli_model_flag"`
	ClaudeCLISystemFlag string   `toml:"claude_cli_system_flag"`
	ClaudeCLISchemaFlag string   `toml:"claude_cli_schema_flag"`
}

type CodexCLIConfigSection struct {
	CodexCLICommand    string   `toml:"codex_cli_command"`
	CodexCLIArgs       []string `toml:"codex_cli_args"`
	CodexCLIModelFlag  string   `toml:"codex_cli_model_flag"`
	CodexCLISystemFlag string   `toml:"codex_cli_system_flag"`
	CodexCLISchemaFlag string   `toml:"codex_cli_schema_flag"`
}

type GeminiCLIConfigSection struct {
	GeminiCLICommand    string   `toml:"gemini_cli_command"`
	GeminiCLIArgs       []string `toml:"gemini_cli_args"`
	GeminiCLIModelFlag  string   `toml:"gemini_cli_model_flag"`
	GeminiCLISystemFlag string   `toml:"gemini_cli_system_flag"`
	GeminiCLISchemaFlag string   `toml:"gemini_cli_schema_flag"`
}

type Config struct {
	CoreConfig
	ProviderTransportConfig
	ShellConfig
	IngestConfig
	LogAnalysisConfigSection
	SummarizeConfigSection
	ClaudeCLIConfigSection
	CodexCLIConfigSection
	GeminiCLIConfigSection
}

func Default() Config {
	return Config{
		CoreConfig: CoreConfig{
			DefaultModel:           "gpt-4o-mini",
			DefaultProvider:        "openai",
			RequestTimeout:         60,
			ChatCacheLength:        100,
			Stream:                 true,
			PrettifyMarkdown:       true,
			DefaultColor:           "cyan",
			DefaultExecuteShellCmd: false,
			LogToDB:                true,
			Temperature:            0,
			TopP:                   1,
		},
		ProviderTransportConfig: ProviderTransportConfig{
			OpenAITransport:    "api",
			AnthropicTransport: "api",
			GeminiTransport:    "api",
		},
		ShellConfig: ShellConfig{
			ShellSandboxImage: "docker.io/library/ubuntu:24.04",
		},
		IngestConfig: IngestConfig{
			InputReductionMode: "truncate",
			DefaultLogProfile:  "",
		},
		LogAnalysisConfigSection: LogAnalysisConfigSection{
			LogAnalysisMarkdown: false,
		},
		SummarizeConfigSection: SummarizeConfigSection{
			SummarizeChunkTokens:       800,
			SummarizeChunkOverlapLines: 5,
		},
		ClaudeCLIConfigSection: ClaudeCLIConfigSection{
			ClaudeCLICommand:    "claude",
			ClaudeCLIArgs:       []string{"-p", "--output-format", "text"},
			ClaudeCLIModelFlag:  "--model",
			ClaudeCLISystemFlag: "--system-prompt",
			ClaudeCLISchemaFlag: "--json-schema",
		},
		CodexCLIConfigSection: CodexCLIConfigSection{
			CodexCLICommand: "codex",
			CodexCLIArgs:    []string{"exec"},
		},
		GeminiCLIConfigSection: GeminiCLIConfigSection{
			GeminiCLICommand:   "gemini",
			GeminiCLIArgs:      []string{"-p"},
			GeminiCLIModelFlag: "--model",
		},
	}
}

func Load() (Config, error) {
	cfg := Default()
	if path := ConfigFilePath(); fileExists(path) {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return Config{}, err
		}
	}
	applyEnvOverrides(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(ConfigFilePath(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(cfg)
}

func ConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "knotical")
	}
	return filepath.Join(base, "knotical")
}

func ConfigFilePath() string  { return filepath.Join(ConfigDir(), "config.toml") }
func KeysFilePath() string    { return filepath.Join(ConfigDir(), "keys.json") }
func LogsDBPath() string      { return filepath.Join(ConfigDir(), "logs.db") }
func ChatCacheDir() string    { return filepath.Join(ConfigDir(), "chat_cache") }
func RolesDir() string        { return filepath.Join(ConfigDir(), "roles") }
func TemplatesDir() string    { return filepath.Join(ConfigDir(), "templates") }
func FragmentsDir() string    { return filepath.Join(ConfigDir(), "fragments") }
func CacheDir() string        { return filepath.Join(ConfigDir(), "cache") }
func LastSessionPath() string { return filepath.Join(ConfigDir(), "last_session.txt") }
func AliasesFilePath() string { return filepath.Join(ConfigDir(), "aliases.json") }

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

type ProviderSettings struct {
	DefaultModel    string
	DefaultProvider string
	RequestTimeout  time.Duration
}

type ProviderCapabilities = provider.Capabilities

type ProviderRuntime struct {
	Name         string
	Transport    string
	BaseURL      string
	CLI          CLIProviderConfig
	Capabilities ProviderCapabilities
}

type ProviderEndpoint struct {
	Transport string
	BaseURL   string
	CLI       CLIProviderConfig
}

type ShellSettings struct {
	ExecuteMode string
	Runtime     string
	Image       string
	Network     bool
	Write       bool
}

type IngestSettings struct {
	MaxInputBytes      int
	MaxInputLines      int
	MaxInputTokens     int
	InputReductionMode string
	DefaultHeadLines   int
	DefaultTailLines   int
	DefaultSampleLines int
}

type LogAnalysisSettings struct {
	Markdown       bool
	Schema         string
	SystemPrompt   string
	DefaultProfile string
}

type SummarizeSettings struct {
	ChunkTokens       int
	ChunkOverlapLines int
	IntermediateModel string
}

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

func (cfg Config) ProviderSettings() ProviderSettings {
	return ProviderSettings{
		DefaultModel:    cfg.DefaultModel,
		DefaultProvider: cfg.DefaultProvider,
		RequestTimeout:  time.Duration(cfg.RequestTimeout) * time.Second,
	}
}

func (cfg Config) ProviderEndpoint(name string) ProviderEndpoint {
	return ProviderEndpoint{
		Transport: cfg.TransportForProvider(name),
		BaseURL:   cfg.BaseURLForProvider(name),
		CLI:       cfg.CLIConfigForProvider(name),
	}
}

func (cfg Config) ProviderRuntime(name string) ProviderRuntime {
	endpoint := cfg.ProviderEndpoint(name)
	caps := provider.CapabilitiesForTransport(name, endpoint.Transport)
	return ProviderRuntime{
		Name:      name,
		Transport: endpoint.Transport,
		BaseURL:   endpoint.BaseURL,
		CLI:       endpoint.CLI,
		Capabilities: ProviderCapabilities{
			NativeStreaming:    caps.NativeStreaming,
			NativeConversation: caps.NativeConversation,
			NativeSchema:       caps.NativeSchema,
			ModelListing:       caps.ModelListing,
		},
	}
}

func (cfg Config) ShellSettings() ShellSettings {
	return ShellSettings{
		ExecuteMode: cfg.ShellExecuteMode,
		Runtime:     cfg.ShellSandboxRuntime,
		Image:       cfg.ShellSandboxImage,
		Network:     cfg.ShellSandboxNetwork,
		Write:       cfg.ShellSandboxWrite,
	}
}

func (cfg Config) IngestSettings() IngestSettings {
	return IngestSettings{
		MaxInputBytes:      cfg.MaxInputBytes,
		MaxInputLines:      cfg.MaxInputLines,
		MaxInputTokens:     cfg.MaxInputTokens,
		InputReductionMode: cfg.InputReductionMode,
		DefaultHeadLines:   cfg.DefaultHeadLines,
		DefaultTailLines:   cfg.DefaultTailLines,
		DefaultSampleLines: cfg.DefaultSampleLines,
	}
}

func (cfg Config) LogAnalysisSettings() LogAnalysisSettings {
	return LogAnalysisSettings{
		Markdown:       cfg.LogAnalysisMarkdown,
		Schema:         cfg.LogAnalysisSchema,
		SystemPrompt:   cfg.LogAnalysisSystemPrompt,
		DefaultProfile: cfg.DefaultLogProfile,
	}
}

func (cfg Config) SummarizeSettings() SummarizeSettings {
	return SummarizeSettings{
		ChunkTokens:       cfg.SummarizeChunkTokens,
		ChunkOverlapLines: cfg.SummarizeChunkOverlapLines,
		IntermediateModel: cfg.SummarizeIntermediateModel,
	}
}

func (cfg Config) BaseURLForProvider(name string) string {
	switch name {
	case "openai":
		return cfg.OpenAIBaseURL
	case "anthropic":
		return cfg.AnthropicBaseURL
	case "gemini":
		return cfg.GeminiBaseURL
	case "ollama":
		if cfg.OllamaBaseURL != "" {
			return cfg.OllamaBaseURL
		}
		return cfg.APIBaseURL
	default:
		return ""
	}
}

func (cfg Config) TransportForProvider(name string) string {
	switch name {
	case "openai":
		return normalizedTransport(cfg.OpenAITransport)
	case "anthropic":
		return normalizedTransport(cfg.AnthropicTransport)
	case "gemini":
		return normalizedTransport(cfg.GeminiTransport)
	default:
		return "api"
	}
}

func normalizedTransport(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cli":
		return "cli"
	default:
		return "api"
	}
}

type CLIProviderConfig struct {
	Command    string
	Args       []string
	ModelFlag  string
	SystemFlag string
	SchemaFlag string
}

func (cfg Config) CLIConfigForProvider(name string) CLIProviderConfig {
	switch name {
	case "anthropic":
		return CLIProviderConfig{
			Command:    cfg.ClaudeCLICommand,
			Args:       append([]string(nil), cfg.ClaudeCLIArgs...),
			ModelFlag:  cfg.ClaudeCLIModelFlag,
			SystemFlag: cfg.ClaudeCLISystemFlag,
			SchemaFlag: cfg.ClaudeCLISchemaFlag,
		}
	case "openai":
		return CLIProviderConfig{
			Command:    cfg.CodexCLICommand,
			Args:       append([]string(nil), cfg.CodexCLIArgs...),
			ModelFlag:  cfg.CodexCLIModelFlag,
			SystemFlag: cfg.CodexCLISystemFlag,
			SchemaFlag: cfg.CodexCLISchemaFlag,
		}
	case "gemini":
		return CLIProviderConfig{
			Command:    cfg.GeminiCLICommand,
			Args:       append([]string(nil), cfg.GeminiCLIArgs...),
			ModelFlag:  cfg.GeminiCLIModelFlag,
			SystemFlag: cfg.GeminiCLISystemFlag,
			SchemaFlag: cfg.GeminiCLISchemaFlag,
		}
	default:
		return CLIProviderConfig{}
	}
}
