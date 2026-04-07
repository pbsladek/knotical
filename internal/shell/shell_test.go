package shell

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestShellCommandUsesEnvShellOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-specific")
	}
	t.Setenv("SHELL", "/bin/zsh")
	name, args := shellCommand("echo hi")
	if name != "/bin/zsh" {
		t.Fatalf("expected env shell, got %q", name)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "echo hi" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestShellCommandFallsBackToShOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-specific")
	}
	t.Setenv("SHELL", "")
	name, args := shellCommand("echo hi")
	if name != "sh" {
		t.Fatalf("expected sh fallback, got %q", name)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "echo hi" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestDetectShellUsesEnvName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-specific")
	}
	t.Setenv("SHELL", "/bin/bash")
	if got := DetectShell(); got != "bash" {
		t.Fatalf("unexpected shell name: %q", got)
	}
}

func TestShellSystemPromptIncludesOSAndShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	prompt := ShellSystemPrompt()
	if !strings.Contains(prompt, runtime.GOOS) || !strings.Contains(prompt, "zsh") {
		t.Fatalf("unexpected shell prompt: %q", prompt)
	}
}

func TestPromptActionParsesHostExecution(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	defer reader.Close()
	if _, err := writer.WriteString("h\n"); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	_ = writer.Close()

	oldStdin := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = oldStdin }()

	action, err := PromptAction(PromptOptions{HasSandbox: true})
	if err != nil {
		t.Fatalf("PromptAction failed: %v", err)
	}
	if action != ActionExecuteHost {
		t.Fatalf("expected host action, got %q", action)
	}
}

func TestConfirmRiskyExecutionRejectsByDefault(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	defer reader.Close()
	if _, err := writer.WriteString("\n"); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	_ = writer.Close()

	oldStdin := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = oldStdin }()

	ok, err := ConfirmRiskyExecution(ExecutionModeHost, RiskReport{
		HighRisk: true,
		Reasons:  []string{"uses sudo"},
	})
	if err != nil {
		t.Fatalf("ConfirmRiskyExecution failed: %v", err)
	}
	if ok {
		t.Fatal("expected default confirmation to reject execution")
	}
}

func TestAnalyzeCommandFlagsHighRiskPatterns(t *testing.T) {
	report := AnalyzeCommand("sudo rm -rf /tmp/data && echo done")
	if !report.HighRisk {
		t.Fatal("expected high-risk report")
	}
	if len(report.Reasons) == 0 {
		t.Fatal("expected risk reasons")
	}
}

func TestParseSimpleCommandSupportsQuotedArgs(t *testing.T) {
	name, args, err := ParseSimpleCommand(`echo "hello world" 'again'`)
	if err != nil {
		t.Fatalf("ParseSimpleCommand failed: %v", err)
	}
	if name != "echo" {
		t.Fatalf("unexpected command name: %q", name)
	}
	if len(args) != 2 || args[0] != "hello world" || args[1] != "again" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestParseSimpleCommandRejectsShellOperators(t *testing.T) {
	if _, _, err := ParseSimpleCommand("echo hi | cat"); err == nil {
		t.Fatal("expected shell operator rejection")
	}
}

func TestParseSimpleCommandRejectsNonAllowlistedExecutables(t *testing.T) {
	if _, _, err := ParseSimpleCommand(`python -c "print(1)"`); err == nil {
		t.Fatal("expected non-allowlisted executable to fail")
	}
}

func TestParseSimpleCommandRejectsPathQualifiedCommands(t *testing.T) {
	if _, _, err := ParseSimpleCommand("/bin/echo hello"); err == nil {
		t.Fatal("expected path-qualified command to fail")
	}
}

func TestParseSimpleCommandAllowsReadOnlyGitSubcommands(t *testing.T) {
	name, args, err := ParseSimpleCommand("git status --short")
	if err != nil {
		t.Fatalf("ParseSimpleCommand failed: %v", err)
	}
	if name != "git" || len(args) < 1 || args[0] != "status" {
		t.Fatalf("unexpected parsed git command: %q %#v", name, args)
	}
}

func TestParseSimpleCommandRejectsMutableGitSubcommands(t *testing.T) {
	if _, _, err := ParseSimpleCommand("git checkout main"); err == nil {
		t.Fatal("expected mutable git subcommand to fail")
	}
}

func TestSandboxCommandArgsUseSafeDefaults(t *testing.T) {
	args, err := sandboxCommandArgs("docker", ExecutionRequest{
		Command: "echo hi",
		Mode:    ExecutionModeSandbox,
		Sandbox: SandboxConfig{
			Runtime: "docker",
			Image:   "ubuntu:24.04",
			Workdir: t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("sandboxCommandArgs failed: %v", err)
	}
	got := strings.Join(args, " ")
	for _, want := range []string{"run --rm -i", "--read-only", "--tmpfs /tmp", "--network none", "ubuntu:24.04 sh -lc echo hi"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected sandbox args to contain %q, got %q", want, got)
		}
	}
}

func TestResolveSandboxRuntimeKeepsExplicitRuntime(t *testing.T) {
	if got := ResolveSandboxRuntime("podman"); got != "podman" {
		t.Fatalf("expected explicit runtime to be preserved, got %q", got)
	}
}
