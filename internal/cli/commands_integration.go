package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/shell"
)

func newInstallIntegrationCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install-integration",
		Short: "Install shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			shellName := shell.DetectShell()
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			var rcFile string
			var snippet string
			binaryPath, err := shellIntegrationBinary()
			if err != nil {
				return err
			}
			quotedBinary := shellSingleQuote(binaryPath)
			switch shellName {
			case "zsh":
				rcFile = filepath.Join(home, ".zshrc")
				snippet = fmt.Sprintf(`
# knotical shell integration
_knotical_widget() {
  local result
  result=$(%s -s "$BUFFER" 2>/dev/null)
  if [ -n "$result" ]; then
    BUFFER="$result"
    CURSOR=${#BUFFER}
  fi
  zle redisplay
}
zle -N _knotical_widget
bindkey '^L' _knotical_widget
`, quotedBinary)
			case "bash":
				rcFile = filepath.Join(home, ".bashrc")
				snippet = fmt.Sprintf(`
# knotical shell integration
_knotical_bind() {
  local result
  result=$(%s -s "$READLINE_LINE" 2>/dev/null)
  if [ -n "$result" ]; then
    READLINE_LINE="$result"
    READLINE_POINT=${#READLINE_LINE}
  fi
}
bind -x '"\C-l": _knotical_bind'
`, quotedBinary)
			default:
				return fmt.Errorf("unsupported shell %q", shellName)
			}
			content, err := os.ReadFile(rcFile)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if strings.Contains(string(content), "_knotical_widget") || strings.Contains(string(content), "_knotical_bind") {
				output.Warn("Shell integration already installed.")
				return nil
			}
			file, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.WriteString(snippet)
			return err
		},
	}
}

func shellIntegrationBinary() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	return path, nil
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
