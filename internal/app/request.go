package app

import "github.com/pbsladek/knotical/internal/shell"

type PromptInput struct {
	PromptText string
	StdinText  string
	StdinMode  string
	StdinLabel string
}

type PipelineInput struct {
	Profile                    string
	Transforms                 []string
	NoPipeline                 bool
	Clean                      bool
	Dedupe                     bool
	Unique                     bool
	K8s                        bool
	MaxInputBytes              int
	MaxInputLines              int
	MaxInputTokens             int
	InputReduction             string
	SummarizeChunkTokens       int
	SummarizeIntermediateModel string
	HeadLines                  int
	TailLines                  int
	SampleLines                int
}

type ModeOptions struct {
	AnalyzeLogs   bool
	Shell         bool
	DescribeShell bool
	Code          bool
	NoMD          bool
	Extract       bool
}

type SessionOptions struct {
	Chat         string
	Repl         string
	ContinueLast bool
}

type TemplateOptions struct {
	Role      string
	Template  string
	Fragments []string
	Save      string
}

type SamplingOptions struct {
	Provider    string
	Model       string
	System      string
	Temperature float64
	Schema      string
	TopP        float64
	NoStream    bool
}

type RunOptions struct {
	Cache       bool
	Interaction bool
	Log         bool
	NoLog       bool
}

type ShellOptions struct {
	ExecuteMode     shell.ExecutionMode
	ForceRiskyShell bool
	SandboxRuntime  string
	SandboxImage    string
	SandboxNetwork  bool
	SandboxWrite    bool
}

type Request struct {
	PromptInput
	PipelineInput
	ModeOptions
	SessionOptions
	TemplateOptions
	SamplingOptions
	RunOptions
	ShellOptions
}
