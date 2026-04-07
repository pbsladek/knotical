package config

import "time"

type ProviderSettings struct {
	DefaultModel    string
	DefaultProvider string
	RequestTimeout  time.Duration
}

type ShellSettings struct {
	ExecuteMode string
	Runtime     string
	Image       string
	Network     bool
	Write       bool
}

type IngestSettings struct {
	MaxInputBytes      int
	MaxInputLines      int
	MaxInputTokens     int
	InputReductionMode string
	DefaultHeadLines   int
	DefaultTailLines   int
	DefaultSampleLines int
}

type LogAnalysisSettings struct {
	Markdown       bool
	Schema         string
	SystemPrompt   string
	DefaultProfile string
}

type SummarizeSettings struct {
	ChunkTokens       int
	ChunkOverlapLines int
	IntermediateModel string
}

func (cfg Config) ProviderSettings() ProviderSettings {
	return ProviderSettings{
		DefaultModel:    cfg.DefaultModel,
		DefaultProvider: cfg.DefaultProvider,
		RequestTimeout:  time.Duration(cfg.RequestTimeout) * time.Second,
	}
}

func (cfg Config) ShellSettings() ShellSettings {
	return ShellSettings{
		ExecuteMode: cfg.ShellExecuteMode,
		Runtime:     cfg.ShellSandboxRuntime,
		Image:       cfg.ShellSandboxImage,
		Network:     cfg.ShellSandboxNetwork,
		Write:       cfg.ShellSandboxWrite,
	}
}

func (cfg Config) IngestSettings() IngestSettings {
	return IngestSettings{
		MaxInputBytes:      cfg.MaxInputBytes,
		MaxInputLines:      cfg.MaxInputLines,
		MaxInputTokens:     cfg.MaxInputTokens,
		InputReductionMode: cfg.InputReductionMode,
		DefaultHeadLines:   cfg.DefaultHeadLines,
		DefaultTailLines:   cfg.DefaultTailLines,
		DefaultSampleLines: cfg.DefaultSampleLines,
	}
}

func (cfg Config) LogAnalysisSettings() LogAnalysisSettings {
	return LogAnalysisSettings{
		Markdown:       cfg.LogAnalysisMarkdown,
		Schema:         cfg.LogAnalysisSchema,
		SystemPrompt:   cfg.LogAnalysisSystemPrompt,
		DefaultProfile: cfg.DefaultLogProfile,
	}
}

func (cfg Config) SummarizeSettings() SummarizeSettings {
	return SummarizeSettings{
		ChunkTokens:       cfg.SummarizeChunkTokens,
		ChunkOverlapLines: cfg.SummarizeChunkOverlapLines,
		IntermediateModel: cfg.SummarizeIntermediateModel,
	}
}
