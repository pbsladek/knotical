package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultModel           string  `toml:"default_model"`
	DefaultProvider        string  `toml:"default_provider"`
	APIBaseURL             string  `toml:"api_base_url"`
	OpenAIBaseURL          string  `toml:"openai_base_url"`
	AnthropicBaseURL       string  `toml:"anthropic_base_url"`
	GeminiBaseURL          string  `toml:"gemini_base_url"`
	OllamaBaseURL          string  `toml:"ollama_base_url"`
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

func Default() Config {
	return Config{
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
	return cfg, nil
}

func Save(cfg Config) error {
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
	if v := os.Getenv("KNOTICAL_DEFAULT_MODEL"); v != "" {
		cfg.DefaultModel = v
	}
	if v := os.Getenv("KNOTICAL_DEFAULT_PROVIDER"); v != "" {
		cfg.DefaultProvider = v
	}
	if v := os.Getenv("KNOTICAL_API_BASE_URL"); v != "" {
		cfg.APIBaseURL = v
	}
	if v := os.Getenv("KNOTICAL_OPENAI_BASE_URL"); v != "" {
		cfg.OpenAIBaseURL = v
	}
	if v := os.Getenv("KNOTICAL_ANTHROPIC_BASE_URL"); v != "" {
		cfg.AnthropicBaseURL = v
	}
	if v := os.Getenv("KNOTICAL_GEMINI_BASE_URL"); v != "" {
		cfg.GeminiBaseURL = v
	}
	if v := os.Getenv("KNOTICAL_OLLAMA_BASE_URL"); v != "" {
		cfg.OllamaBaseURL = v
	}
	if v := os.Getenv("KNOTICAL_REQUEST_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RequestTimeout = n
		}
	}
	if v := os.Getenv("KNOTICAL_STREAM"); v != "" {
		cfg.Stream = parseBool(v, cfg.Stream)
	}
	if v := os.Getenv("KNOTICAL_PRETTIFY_MARKDOWN"); v != "" {
		cfg.PrettifyMarkdown = parseBool(v, cfg.PrettifyMarkdown)
	}
	if v := os.Getenv("KNOTICAL_LOG_TO_DB"); v != "" {
		cfg.LogToDB = parseBool(v, cfg.LogToDB)
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
