package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallIntegrationCommandInstallsAndWarns(t *testing.T) {
	setupCLIConfigHome(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir failed: %v", err)
	}
	t.Setenv("SHELL", "/bin/zsh")
	outputBuffer := captureDefaultOutput(t)

	cmd := newInstallIntegrationCommand()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install integration failed: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("read zshrc failed: %v", err)
	}
	if !strings.Contains(string(payload), "_knotical_widget") {
		t.Fatalf("expected zsh integration snippet, got %q", string(payload))
	}
	binaryPath, err := shellIntegrationBinary()
	if err != nil {
		t.Fatalf("shellIntegrationBinary failed: %v", err)
	}
	if !strings.Contains(string(payload), shellSingleQuote(binaryPath)) {
		t.Fatalf("expected quoted binary path in snippet, got %q", string(payload))
	}
	info, err := os.Stat(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("stat zshrc failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected secure rc file mode, got %o", got)
	}

	cmd = newInstallIntegrationCommand()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second install integration failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "already installed") {
		t.Fatalf("expected already-installed warning, got %q", outputBuffer.String())
	}
}

func TestShellSingleQuote(t *testing.T) {
	got := shellSingleQuote("/tmp/it's knotical")
	if got != `'/tmp/it'\''s knotical'` {
		t.Fatalf("unexpected quoted value: %q", got)
	}
}
