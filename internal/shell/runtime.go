package shell

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

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
