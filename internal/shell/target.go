package shell

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

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
