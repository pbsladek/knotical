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
	tokenStarted := false

	flush := func() {
		if !tokenStarted && current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
		tokenStarted = false
	}

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
			tokenStarted = true
		case r == '\\' && !inSingle:
			escaped = true
			tokenStarted = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			tokenStarted = true
		case r == '"' && !inSingle:
			inDouble = !inDouble
			tokenStarted = true
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
			tokenStarted = true
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
	case "status", "log", "show", "diff", "rev-parse", "ls-files", "grep":
		return validateSafeGitReadOnlyArgs(args[1:])
	case "branch":
		return validateSafeGitBranch(args[1:])
	case "remote":
		return validateSafeGitRemote(args[1:])
	default:
		return fmt.Errorf("safe execution does not allow git subcommand %q", args[0])
	}
}

func validateSafeGitReadOnlyArgs(args []string) error {
	for _, arg := range args {
		if isDisallowedSafeGitArg(arg) {
			return fmt.Errorf("safe git execution does not allow option %q", arg)
		}
	}
	return nil
}

func validateSafeGitBranch(args []string) error {
	if err := validateSafeGitReadOnlyArgs(args); err != nil {
		return err
	}
	for _, arg := range args {
		if !isSafeGitBranchArg(arg) {
			return fmt.Errorf("safe git branch execution only allows listing options; %q is not permitted", arg)
		}
	}
	return nil
}

func validateSafeGitRemote(args []string) error {
	if err := validateSafeGitReadOnlyArgs(args); err != nil {
		return err
	}
	if len(args) == 0 {
		return nil
	}
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--verbose") {
		return nil
	}
	if args[0] != "get-url" {
		return fmt.Errorf("safe git remote execution only allows listing and get-url")
	}
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") && arg != "--push" && arg != "--all" {
			return fmt.Errorf("safe git remote get-url does not allow option %q", arg)
		}
	}
	return nil
}

func isDisallowedSafeGitArg(arg string) bool {
	if arg == "" {
		return false
	}
	switch arg {
	case "-h", "--help", "-?", "-o", "--output", "--ext-diff", "--textconv", "-O", "--open-files-in-pager", "--paginate", "--exec-path", "--git-dir", "--work-tree":
		return true
	}
	for _, prefix := range []string{
		"--output=",
		"--open-files-in-pager=",
		"--exec-path=",
		"--git-dir=",
		"--work-tree=",
	} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func isSafeGitBranchArg(arg string) bool {
	switch {
	case arg == "":
		return true
	case arg == "--all", arg == "-a", arg == "--remotes", arg == "-r", arg == "--list", arg == "-l":
		return true
	case arg == "--verbose", arg == "-v", arg == "-vv", arg == "--show-current", arg == "--no-color":
		return true
	case strings.HasPrefix(arg, "--color="), strings.HasPrefix(arg, "--format="), strings.HasPrefix(arg, "--sort="):
		return true
	default:
		return false
	}
}
