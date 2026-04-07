package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
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
