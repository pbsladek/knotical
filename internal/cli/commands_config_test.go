package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/config"
)

func TestWriteConfigFileCreatesDirectoriesAndUsesSecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	cfg := config.Default()
	cfg.DefaultModel = "claude-sonnet-4-5"

	if err := writeConfigFile(path, cfg, false); err != nil {
		t.Fatalf("writeConfigFile failed: %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if !strings.Contains(string(payload), `default_model = "claude-sonnet-4-5"`) {
		t.Fatalf("unexpected config contents: %q", string(payload))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", got)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir failed: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("expected 0700 directory permissions, got %o", got)
	}
}

func TestWriteConfigFileRefusesOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("default_model = \"first\"\n"), 0o600); err != nil {
		t.Fatalf("seed config failed: %v", err)
	}

	err := writeConfigFile(path, config.Default(), false)
	if err == nil {
		t.Fatal("expected overwrite refusal")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected os.ErrExist, got %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if string(payload) != "default_model = \"first\"\n" {
		t.Fatalf("expected existing config to remain unchanged, got %q", string(payload))
	}
}

func TestWriteConfigFileOverwritesWithForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("default_model = \"first\"\n"), 0o600); err != nil {
		t.Fatalf("seed config failed: %v", err)
	}

	cfg := config.Default()
	cfg.DefaultModel = "gpt-5"
	if err := writeConfigFile(path, cfg, true); err != nil {
		t.Fatalf("writeConfigFile with force failed: %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if !strings.Contains(string(payload), `default_model = "gpt-5"`) {
		t.Fatalf("expected overwritten config contents, got %q", string(payload))
	}
}
