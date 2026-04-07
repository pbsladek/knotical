package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/ingest"
	"github.com/pbsladek/knotical/internal/shell"
)

func applyShellDefaults(req Request, cfg config.Config) Request {
	if req.Shell {
		req = applyShellExecutionDefaults(req, cfg.ShellSettings())
	}
	return applyInputDefaults(req, cfg)
}

func applyShellExecutionDefaults(req Request, shellCfg config.ShellSettings) Request {
	req.ExecuteMode = defaultExecutionMode(req.ExecuteMode, shellCfg.ExecuteMode)
	req.SandboxRuntime = defaultString(req.SandboxRuntime, shellCfg.Runtime)
	req.SandboxImage = defaultString(req.SandboxImage, shellCfg.Image)
	req.SandboxNetwork = defaultBool(req.SandboxNetwork, shellCfg.Network)
	req.SandboxWrite = defaultBool(req.SandboxWrite, shellCfg.Write)
	return req
}

func applyInputDefaults(req Request, cfg config.Config) Request {
	req = applyIngestDefaults(req, cfg.IngestSettings())
	req = applySummarizeDefaults(req, cfg.SummarizeSettings())
	return applyAnalyzeLogDefaults(req, cfg.LogAnalysisSettings())
}

func applyIngestDefaults(req Request, ingestCfg config.IngestSettings) Request {
	req.MaxInputBytes = defaultInt(req.MaxInputBytes, ingestCfg.MaxInputBytes)
	req.MaxInputLines = defaultInt(req.MaxInputLines, ingestCfg.MaxInputLines)
	req.MaxInputTokens = defaultInt(req.MaxInputTokens, ingestCfg.MaxInputTokens)
	req.InputReduction = defaultString(req.InputReduction, ingestCfg.InputReductionMode)
	req.HeadLines = defaultInt(req.HeadLines, ingestCfg.DefaultHeadLines)
	req.TailLines = defaultInt(req.TailLines, ingestCfg.DefaultTailLines)
	req.SampleLines = defaultInt(req.SampleLines, ingestCfg.DefaultSampleLines)
	return req
}

func applySummarizeDefaults(req Request, summarizeCfg config.SummarizeSettings) Request {
	if req.SummarizeChunkTokens == 0 && summarizeCfg.ChunkTokens > 0 {
		req.SummarizeChunkTokens = summarizeCfg.ChunkTokens
	}
	if req.SummarizeIntermediateModel == "" && strings.TrimSpace(summarizeCfg.IntermediateModel) != "" {
		req.SummarizeIntermediateModel = strings.TrimSpace(summarizeCfg.IntermediateModel)
	}
	return req
}

func applyAnalyzeLogDefaults(req Request, logCfg config.LogAnalysisSettings) Request {
	if !req.AnalyzeLogs {
		return req
	}
	if req.Profile == "" && strings.TrimSpace(logCfg.DefaultProfile) != "" {
		req.Profile = strings.TrimSpace(logCfg.DefaultProfile)
	}
	if req.Schema == "" && strings.TrimSpace(logCfg.Schema) != "" {
		req.Schema = strings.TrimSpace(logCfg.Schema)
	}
	if req.StdinLabel == "" || req.StdinLabel == "input" {
		req.StdinLabel = "logs"
	}
	return req
}

func defaultInt(current int, fallback int) int {
	if current == 0 && fallback > 0 {
		return fallback
	}
	return current
}

func defaultString(current string, fallback string) string {
	if current == "" && fallback != "" {
		return fallback
	}
	return current
}

func defaultBool(current bool, fallback bool) bool {
	if !current && fallback {
		return true
	}
	return current
}

func defaultExecutionMode(current shell.ExecutionMode, fallback string) shell.ExecutionMode {
	if current == "" && fallback != "" {
		return shell.ExecutionMode(fallback)
	}
	return current
}

func requestPipelineOptions(req Request) ingest.PipelineOptions {
	pipeline := req.PipelineInput
	shorthands := make([]string, 0, 4)
	if pipeline.Clean {
		shorthands = append(shorthands, "clean")
	}
	if pipeline.Dedupe {
		shorthands = append(shorthands, "dedupe")
	}
	if pipeline.Unique {
		shorthands = append(shorthands, "unique")
	}
	if pipeline.K8s {
		shorthands = append(shorthands, "k8s")
	}
	return ingest.PipelineOptions{
		Profile:    pipeline.Profile,
		Shorthands: shorthands,
		Transforms: append([]string(nil), pipeline.Transforms...),
		NoPipeline: pipeline.NoPipeline,
	}
}

func applySchemaFallbackInstruction(systemPrompt string, schemaValue map[string]any, caps config.ProviderCapabilities) string {
	if schemaValue == nil || caps.NativeSchema {
		return systemPrompt
	}
	schemaJSON, _ := json.Marshal(schemaValue)
	instruction := fmt.Sprintf("Respond with valid JSON matching this schema: %s. No other text.", string(schemaJSON))
	if systemPrompt != "" {
		return systemPrompt + "\n\n" + instruction
	}
	return instruction
}

func samplingPointers(temperature float64, topP float64) (*float64, *float64) {
	var tempPtr *float64
	if temperature > 0 {
		tempPtr = &temperature
	}
	var topPPtr *float64
	if topP != 1 {
		topPPtr = &topP
	}
	return tempPtr, topPPtr
}

func extractCodeBlock(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	inBlock := false
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inBlock {
				break
			}
			inBlock = true
			continue
		}
		if inBlock {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return text
	}
	return strings.Join(lines, "\n")
}

func (s *Service) preparePrompt(prompt string, req Request) string {
	if !req.DescribeShell {
		return prompt
	}
	return "Explain what this shell command does:\n\n" + strings.TrimSpace(prompt)
}
