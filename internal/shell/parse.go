package shell

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ParseSimpleCommand(command string) (string, []string, error) {
	for _, marker := range []string{"|", "&", ";", ">", "<", "`", "$(", "\n", "\r"} {
		if strings.Contains(command, marker) {
			return "", nil, fmt.Errorf("safe execution does not allow shell operator %q", marker)
		}
	}
	tokens, err := splitCommand(command)
	if err != nil {
		return "", nil, err
	}
	if len(tokens) == 0 {
		return "", nil, fmt.Errorf("empty shell command")
	}
	if err := validateSafeCommand(tokens[0], tokens[1:]); err != nil {
		return "", nil, err
	}
	return tokens[0], tokens[1:], nil
}

func splitCommand(command string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escaped || inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted string in command")
	}
	flush()
	return tokens, nil
}

func validateSafeCommand(name string, args []string) error {
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("safe execution does not allow path-qualified commands")
	}
	base := filepath.Base(name)
	switch base {
	case "cat", "echo", "printf", "pwd", "uname", "whoami", "date", "ls", "head", "tail", "wc", "which", "rg", "grep":
		return nil
	case "git":
		return validateSafeGit(args)
	default:
		return fmt.Errorf("safe execution only allows read-only commands; %q is not permitted", base)
	}
}

func validateSafeGit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("safe git execution requires a subcommand")
	}
	switch args[0] {
	case "status", "log", "show", "diff", "branch", "rev-parse", "remote", "ls-files", "grep":
		return nil
	default:
		return fmt.Errorf("safe execution does not allow git subcommand %q", args[0])
	}
}
