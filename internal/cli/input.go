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

type promptSource struct {
	instructionText string
	stdinText       string
}

func readPromptSource(opts rootOptions) (promptSource, error) {
	instructionText := strings.TrimSpace(strings.Join(opts.Prompt, " "))
	stdinText, hasStdin, err := readStdinPrompt()
	if err != nil {
		return promptSource{}, err
	}
	if opts.Editor {
		if hasStdin {
			return promptSource{}, fmt.Errorf("--editor cannot be combined with piped stdin")
		}
		instructionText, err = openEditorForPrompt()
		if err != nil {
			return promptSource{}, err
		}
	}
	if hasStdin && stdinText == "" {
		return promptSource{}, fmt.Errorf("empty prompt from stdin")
	}
	if instructionText == "" && !hasStdin {
		return promptSource{}, fmt.Errorf("no prompt provided")
	}
	return promptSource{instructionText: instructionText, stdinText: stdinText}, nil
}

func readStdinPrompt() (string, bool, error) {
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
		return "", false, nil
	}
	payload, err := ioReadAll(os.Stdin)
	if err != nil {
		return "", true, err
	}
	return strings.TrimSpace(string(payload)), true, nil
}

func resolveAPIKey(providerName string) (string, error) {
	if providerName == "ollama" {
		return "", nil
	}
	return store.NewKeyManager(config.KeysFilePath()).Require(providerName)
}

func openEditorForPrompt() (string, error) {
	file, err := os.CreateTemp("", "knotical_prompt_*.txt")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()
	defer os.Remove(path)
	if err := openEditorAtPath(path); err != nil {
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

func openEditorAtPath(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	name, args, err := editorCommandArgs(editor, path)
	if err != nil {
		return err
	}
	command := exec.Command(name, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	return nil
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
