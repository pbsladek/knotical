package config

import "testing"

func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	original := Default()
	original.DefaultModel = "gemini-2.5-flash"
	original.DefaultProvider = "gemini"
	original.GeminiTransport = "cli"
	original.RequestTimeout = 90
	original.Stream = false
	original.LogToDB = false
	original.Temperature = 0.4
	original.TopP = 0.8
	original.ShellExecuteMode = "sandbox"
	original.ShellSandboxRuntime = "docker"
	original.ShellSandboxImage = "ubuntu:24.04"
	original.ShellSandboxNetwork = true
	original.ShellSandboxWrite = true
	original.MaxInputBytes = 4096
	original.MaxInputLines = 200
	original.MaxInputTokens = 800
	original.InputReductionMode = "fail"
	original.LogAnalysisMarkdown = true
	original.LogAnalysisSchema = "summary, likely_root_cause"
	original.LogAnalysisSystemPrompt = "custom log prompt"
	original.DefaultLogProfile = "k8s"
	original.SummarizeChunkTokens = 600
	original.SummarizeChunkOverlapLines = 7
	original.SummarizeIntermediateModel = "gpt-4o-mini"
	original.DefaultHeadLines = 20
	original.DefaultTailLines = 30
	original.DefaultSampleLines = 40

	if err := Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.DefaultModel != original.DefaultModel ||
		loaded.DefaultProvider != original.DefaultProvider ||
		loaded.GeminiTransport != original.GeminiTransport ||
		loaded.RequestTimeout != original.RequestTimeout ||
		loaded.Stream != original.Stream ||
		loaded.LogToDB != original.LogToDB ||
		loaded.Temperature != original.Temperature ||
		loaded.TopP != original.TopP ||
		loaded.ShellExecuteMode != original.ShellExecuteMode ||
		loaded.ShellSandboxRuntime != original.ShellSandboxRuntime ||
		loaded.ShellSandboxImage != original.ShellSandboxImage ||
		loaded.ShellSandboxNetwork != original.ShellSandboxNetwork ||
		loaded.ShellSandboxWrite != original.ShellSandboxWrite ||
		loaded.MaxInputBytes != original.MaxInputBytes ||
		loaded.MaxInputLines != original.MaxInputLines ||
		loaded.MaxInputTokens != original.MaxInputTokens ||
		loaded.InputReductionMode != original.InputReductionMode ||
		loaded.LogAnalysisMarkdown != original.LogAnalysisMarkdown ||
		loaded.LogAnalysisSchema != original.LogAnalysisSchema ||
		loaded.LogAnalysisSystemPrompt != original.LogAnalysisSystemPrompt ||
		loaded.DefaultLogProfile != original.DefaultLogProfile ||
		loaded.SummarizeChunkTokens != original.SummarizeChunkTokens ||
		loaded.SummarizeChunkOverlapLines != original.SummarizeChunkOverlapLines ||
		loaded.SummarizeIntermediateModel != original.SummarizeIntermediateModel ||
		loaded.DefaultHeadLines != original.DefaultHeadLines ||
		loaded.DefaultTailLines != original.DefaultTailLines ||
		loaded.DefaultSampleLines != original.DefaultSampleLines {
		t.Fatalf("unexpected config round trip: %+v", loaded)
	}
}
