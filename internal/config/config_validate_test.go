package config

import "testing"

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
