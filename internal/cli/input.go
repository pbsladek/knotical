package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/store"
)

func readPrompt(opts rootOptions) (string, error) {
	if len(opts.Prompt) > 0 {
		return strings.Join(opts.Prompt, " "), nil
	}
	if opts.Editor {
		return openEditorForPrompt()
	}
	stat, err := os.Stdin.Stat()
	if err == nil && stat.Mode()&os.ModeCharDevice == 0 {
		payload, err := ioReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		text := strings.TrimSpace(string(payload))
		if text == "" {
			return "", fmt.Errorf("empty prompt from stdin")
		}
		return text, nil
	}
	return "", fmt.Errorf("no prompt provided")
}

func resolveAPIKey(providerName string) (string, error) {
	if providerName == "ollama" {
		return "", nil
	}
	return store.NewKeyManager(config.KeysFilePath()).Require(providerName)
}

func openEditorForPrompt() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	file, err := os.CreateTemp("", "knotical_prompt_*.txt")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()
	defer os.Remove(path)
	name, args, err := editorCommandArgs(editor, path)
	if err != nil {
		return "", err
	}
	command := exec.Command(name, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return "", err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return "", fmt.Errorf("empty prompt from editor")
	}
	return text, nil
}

func editorCommandArgs(editor string, path string) (string, []string, error) {
	fields := strings.Fields(strings.TrimSpace(editor))
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("empty editor command")
	}
	return fields[0], append(fields[1:], path), nil
}

func ioReadAll(file *os.File) ([]byte, error) {
	return io.ReadAll(file)
}
