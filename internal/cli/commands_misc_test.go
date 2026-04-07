package cli

import "testing"

func setupCLIConfigHome(t *testing.T) {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
}
