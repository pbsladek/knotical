package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	DefaultSandboxImage = "docker.io/library/ubuntu:24.04"
	DefaultWorkspaceDir = "/workspace"
)

type ExecutionMode string

const (
	ExecutionModeHost    ExecutionMode = "host"
	ExecutionModeSafe    ExecutionMode = "safe"
	ExecutionModeSandbox ExecutionMode = "sandbox"
)

type Action string

const (
	ActionExecuteHost    Action = "execute_host"
	ActionExecuteSafe    Action = "execute_safe"
	ActionExecuteSandbox Action = "execute_sandbox"
	ActionDescribe       Action = "describe"
	ActionAbort          Action = "abort"
)

type SandboxConfig struct {
	Runtime string
	Image   string
	Network bool
	Write   bool
	Workdir string
}

type Target struct {
	OS    string
	Shell string
}

type ExecutionRequest struct {
	Command string
	Mode    ExecutionMode
	Sandbox SandboxConfig
}

type PromptOptions struct {
	HasSandbox bool
	Risk       RiskReport
}

type RiskReport struct {
	HighRisk bool
	Reasons  []string
}

var riskPatterns = []struct {
	re     *regexp.Regexp
	reason string
}{
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])sudo([^[:alnum:]_]|$)`), reason: "uses sudo"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])rm([^[:alnum:]_]|$)`), reason: "removes files"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])mv([^[:alnum:]_]|$)`), reason: "moves or renames files"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])(chmod|chown)([^[:alnum:]_]|$)`), reason: "changes permissions or ownership"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])dd([^[:alnum:]_]|$)`), reason: "writes raw block data"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])(mkfs|fdisk|diskutil)([^[:alnum:]_]|$)`), reason: "modifies disks or filesystems"},
	{re: regexp.MustCompile(`(curl|wget)[^|]*\|\s*(sh|bash|zsh)`), reason: "pipes a downloaded script into a shell"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])(scp|ssh)([^[:alnum:]_]|$)`), reason: "accesses remote hosts"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])rsync([^[:alnum:]_]|$)`), reason: "copies files, possibly to remote hosts"},
	{re: regexp.MustCompile("[|;&<>`]|\\$\\("), reason: "uses shell operators or redirection"},
}

func DetectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		parts := strings.Split(shell, "/")
		return parts[len(parts)-1]
	}
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	return "sh"
}

func ShellSystemPrompt() string {
	return ShellSystemPromptForTarget(HostTarget())
}

func SandboxSystemPrompt() string {
	return ShellSystemPromptForTarget(SandboxTarget())
}

func ShellSystemPromptForTarget(target Target) string {
	return fmt.Sprintf(
		"Provide only a single shell command as output with no explanation. Output the command for the current OS (%s) and shell (%s). Do NOT use markdown code blocks.",
		target.OS,
		target.Shell,
	)
}

func HostTarget() Target {
	return Target{OS: runtime.GOOS, Shell: DetectShell()}
}

func SandboxTarget() Target {
	return Target{OS: "linux", Shell: "sh"}
}

func HostCompatibleWithSandbox() bool {
	target := HostTarget()
	return target.OS == "linux" && target.Shell == "sh"
}

func PromptAction(options PromptOptions) (Action, error) {
	if options.Risk.HighRisk {
		fmt.Printf("Warning: %s\n", strings.Join(options.Risk.Reasons, "; "))
	}
	choices := "[H]ost, [S]afe, "
	if options.HasSandbox {
		choices += "[B]andbox, "
	}
	choices += "[D]escribe, [A]bort? "
	fmt.Print(choices)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ActionAbort, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "h", "host":
		return ActionExecuteHost, nil
	case "s", "safe":
		return ActionExecuteSafe, nil
	case "b", "sandbox":
		if options.HasSandbox {
			return ActionExecuteSandbox, nil
		}
		return ActionAbort, nil
	case "d", "describe":
		return ActionDescribe, nil
	default:
		return ActionAbort, nil
	}
}

func ConfirmRiskyExecution(mode ExecutionMode, report RiskReport) (bool, error) {
	if !report.HighRisk {
		return true, nil
	}
	fmt.Printf("Confirm %s execution of risky command (%s)? [y/N] ", mode, strings.Join(report.Reasons, "; "))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func ExecuteCommand(req ExecutionRequest) error {
	switch req.Mode {
	case ExecutionModeHost:
		return executeHost(req.Command)
	case ExecutionModeSafe:
		return executeSafe(req.Command)
	case ExecutionModeSandbox:
		return executeSandbox(req)
	default:
		return fmt.Errorf("unknown shell execution mode %q", req.Mode)
	}
}

func AnalyzeCommand(command string) RiskReport {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return RiskReport{}
	}
	report := RiskReport{}
	seen := map[string]struct{}{}
	for _, pattern := range riskPatterns {
		if pattern.re.MatchString(trimmed) {
			if _, ok := seen[pattern.reason]; ok {
				continue
			}
			seen[pattern.reason] = struct{}{}
			report.Reasons = append(report.Reasons, pattern.reason)
		}
	}
	report.HighRisk = len(report.Reasons) > 0
	return report
}

func SandboxRuntimeAvailable(runtimeName string) bool {
	runtimeName = ResolveSandboxRuntime(runtimeName)
	_, err := exec.LookPath(runtimeName)
	return err == nil
}

func ResolveSandboxRuntime(runtimeName string) string {
	switch runtimeName {
	case "docker", "podman":
		return runtimeName
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
}

func BuildSandboxCommand(req ExecutionRequest) (string, []string, error) {
	runtimeName := ResolveSandboxRuntime(req.Sandbox.Runtime)
	if _, err := exec.LookPath(runtimeName); err != nil {
		return "", nil, fmt.Errorf("%s is not available", runtimeName)
	}
	args, err := sandboxCommandArgs(runtimeName, req)
	if err != nil {
		return "", nil, err
	}
	return runtimeName, args, nil
}

func sandboxCommandArgs(runtimeName string, req ExecutionRequest) ([]string, error) {
	workdir := req.Sandbox.Workdir
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	image := req.Sandbox.Image
	if image == "" {
		image = DefaultSandboxImage
	}
	mountMode := "ro"
	args := []string{"run", "--rm", "-i"}
	if req.Sandbox.Write {
		mountMode = "rw"
	} else {
		args = append(args, "--read-only", "--tmpfs", "/tmp")
	}
	args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,%s", workdir, DefaultWorkspaceDir, mountMode))
	if !req.Sandbox.Network {
		args = append(args, "--network", "none")
	}
	args = append(args, "-w", DefaultWorkspaceDir, image, "sh", "-lc", req.Command)
	_ = runtimeName
	return args, nil
}

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

func executeHost(command string) error {
	name, args := shellCommand(command)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func executeSafe(command string) error {
	name, args, err := ParseSimpleCommand(command)
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func executeSandbox(req ExecutionRequest) error {
	name, args, err := BuildSandboxCommand(req)
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-Command", command}
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell, []string{"-c", command}
	}
	return "sh", []string{"-c", command}
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
