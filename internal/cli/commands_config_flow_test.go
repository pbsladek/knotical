package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/config"
)

func TestConfigCommandShowPathAndEdit(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	cmd := newConfigCommand()
	cmd.SetArgs([]string{"generate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config generate failed: %v", err)
	}
	if _, err := os.Stat(config.ConfigFilePath()); err != nil {
		t.Fatalf("expected generated config file, err=%v", err)
	}

	outputBuffer.Reset()
	t.Setenv("KNOTICAL_DEFAULT_MODEL", "claude-sonnet-4-5")
	cmd = newConfigCommand()
	cmd.SetArgs([]string{"show"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), `default_model = "claude-sonnet-4-5"`) {
		t.Fatalf("expected effective config output, got %q", outputBuffer.String())
	}

	outputBuffer.Reset()
	cmd = newConfigCommand()
	cmd.SetArgs([]string{"path"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config path failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), config.ConfigFilePath()) {
		t.Fatalf("expected config path output, got %q", outputBuffer.String())
	}

	editorScript := filepath.Join(t.TempDir(), "config-editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nfor last; do :; done\nprintf 'default_model = \"gpt-5\"\\n' > \"$last\"\n"), 0o755); err != nil {
		t.Fatalf("write editor script failed: %v", err)
	}
	t.Setenv("EDITOR", editorScript+" --wait")

	cmd = newConfigCommand()
	cmd.SetArgs([]string{"edit"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config edit failed: %v", err)
	}

	payload, err := os.ReadFile(config.ConfigFilePath())
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}
	if !strings.Contains(string(payload), `default_model = "gpt-5"`) {
		t.Fatalf("expected edited config file, got %q", string(payload))
	}
}

func TestConfigCommandGenerateExplicitPathAndForce(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)
	customPath := filepath.Join(t.TempDir(), "custom-config.toml")

	cmd := newConfigCommand()
	cmd.SetArgs([]string{"generate", customPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config generate explicit path failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), customPath) {
		t.Fatalf("expected generated path output, got %q", outputBuffer.String())
	}
	payload, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("read generated config failed: %v", err)
	}
	if !strings.Contains(string(payload), `default_model = "gpt-4o-mini"`) {
		t.Fatalf("expected default config contents, got %q", string(payload))
	}

	cmd = newConfigCommand()
	cmd.SetArgs([]string{"generate", customPath})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected second generate without --force to fail")
	}

	cmd = newConfigCommand()
	cmd.SetArgs([]string{"generate", "--force", customPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config generate --force failed: %v", err)
	}
}
