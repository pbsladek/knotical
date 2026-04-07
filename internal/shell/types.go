package shell

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
