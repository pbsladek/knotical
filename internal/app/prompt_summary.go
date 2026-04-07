package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/ingest"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
)

func (s *Service) summarizeOversizedInput(ctx context.Context, cfg config.Config, req Request, prov provider.Provider, modelID string, ingested ingest.Result) (string, *model.ReductionMetadata, error) {
	inputText := ingested.InputText
	reduction := ingested.Reduction
	if reduction == nil {
		return inputText, reduction, nil
	}
	pipelineInput := req.PipelineInput
	chunkTokens := resolveSummaryChunkTokens(cfg, pipelineInput)
	overlapLines := resolveSummaryOverlapLines(cfg)
	finalModel, finalProvider, err := s.resolveSummaryExecution(ctx, cfg, pipelineInput, prov, modelID)
	if err != nil {
		return "", reduction, err
	}
	chunks := splitSummaryChunks(inputText, chunkTokens, overlapLines)
	if len(chunks) == 0 {
		return inputText, reduction, nil
	}
	finalSummary, err := s.summarizeChunks(ctx, finalProvider, finalModel, chunks)
	if err != nil {
		return "", reduction, err
	}
	if pipelineInput.MaxInputTokens > 0 && ingest.EstimateTokens(finalSummary) > pipelineInput.MaxInputTokens {
		finalSummary = ingest.TruncateToTokenBudget(finalSummary, pipelineInput.MaxInputTokens)
	}
	if finalSummary == "" {
		return "", reduction, fmt.Errorf("summarized input became empty after token truncation")
	}
	reduction.Summarized = true
	reduction.SummaryChunks = len(chunks)
	reduction.IntermediateModel = finalModel
	reduction.Steps = append(reduction.Steps, fmt.Sprintf("summarized:%d", len(chunks)))
	reduction.FinalBytes = len(finalSummary)
	reduction.FinalLines = countSummaryLines(finalSummary)
	reduction.FinalTokens = ingest.EstimateTokens(finalSummary)
	return finalSummary, reduction, nil
}

func resolveSummaryChunkTokens(cfg config.Config, pipelineInput PipelineInput) int {
	chunkTokens := pipelineInput.SummarizeChunkTokens
	if chunkTokens <= 0 {
		chunkTokens = cfg.SummarizeSettings().ChunkTokens
	}
	if chunkTokens <= 0 {
		chunkTokens = 800
	}
	return chunkTokens
}

func resolveSummaryOverlapLines(cfg config.Config) int {
	overlapLines := cfg.SummarizeSettings().ChunkOverlapLines
	if overlapLines < 0 {
		return 0
	}
	return overlapLines
}

func (s *Service) resolveSummaryExecution(ctx context.Context, cfg config.Config, pipelineInput PipelineInput, prov provider.Provider, modelID string) (string, provider.Provider, error) {
	_ = ctx
	if pipelineInput.SummarizeIntermediateModel == "" {
		return modelID, prov, nil
	}
	finalModel := s.resolveAlias(pipelineInput.SummarizeIntermediateModel)
	providerName := provider.DetectProvider(finalModel, cfg.ProviderSettings().DefaultProvider)
	runtimeCfg := cfg.ProviderRuntime(providerName)
	finalProvider, _, err := s.buildConfiguredProvider(cfg, runtimeCfg)
	if err != nil {
		return "", nil, err
	}
	return finalModel, finalProvider, nil
}

func (s *Service) summarizeChunks(ctx context.Context, prov provider.Provider, modelID string, chunks []string) (string, error) {
	summaries := make([]string, 0, len(chunks))
	for idx, chunk := range chunks {
		summary, err := summarizeChunk(ctx, prov, modelID, idx, len(chunks), chunk)
		if err != nil {
			return "", err
		}
		summaries = append(summaries, summary)
	}
	return synthesizeSummaries(ctx, prov, modelID, summaries)
}

func summarizeChunk(ctx context.Context, prov provider.Provider, modelID string, idx int, total int, chunk string) (string, error) {
	resp, err := prov.Complete(ctx, provider.Request{
		Model: modelID,
		Messages: []model.Message{{
			Role: model.RoleUser,
			Content: fmt.Sprintf("Summarize the key operational signals from this log chunk. Capture errors, warnings, repeated patterns, affected components, and likely root-cause clues. Return compact plain text bullets only.\n\nChunk %d/%d:\n%s",
				idx+1, total, chunk),
		}},
		System:    "You summarize operational log chunks for later synthesis. Return only concise plain text bullets.",
		MaxTokens: 1024,
		Stream:    false,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func synthesizeSummaries(ctx context.Context, prov provider.Provider, modelID string, summaries []string) (string, error) {
	finalSummary := strings.Join(summaries, "\n\n")
	if len(summaries) > 1 {
		resp, err := prov.Complete(ctx, provider.Request{
			Model: modelID,
			Messages: []model.Message{{
				Role:    model.RoleUser,
				Content: "Synthesize these log chunk summaries into one compact operational summary. Preserve the strongest evidence, likely root cause clues, affected components, and next investigation steps. Return plain text bullets only.\n\n" + finalSummary,
			}},
			System:    "You synthesize log chunk summaries into one concise operational summary.",
			MaxTokens: 1024,
			Stream:    false,
		})
		if err != nil {
			return "", err
		}
		finalSummary = strings.TrimSpace(resp.Content)
	}
	if finalSummary == "" {
		return "", fmt.Errorf("summarization produced an empty result")
	}
	return finalSummary, nil
}

func composeSummarizedPrompt(req Request, reduction *model.ReductionMetadata, summarized string) string {
	instruction := strings.TrimSpace(req.PromptText)
	if req.StdinMode == ingest.ModeReplace || instruction == "" {
		return summarized
	}
	label := "input"
	if reduction != nil && strings.TrimSpace(reduction.StdinLabel) != "" {
		label = strings.TrimSpace(reduction.StdinLabel)
	}
	return instruction + "\n\n" + label + ":\n" + summarized
}

func splitSummaryChunks(text string, targetTokens int, overlapLines int) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return nil
	}
	if targetTokens <= 0 {
		targetTokens = 800
	}
	chunks := []string{}
	for start := 0; start < len(lines); {
		end := start
		tokenCount := 0
		for end < len(lines) {
			lineTokens := ingest.EstimateTokens(lines[end]) + 1
			if tokenCount+lineTokens > targetTokens && end > start {
				break
			}
			tokenCount += lineTokens
			end++
		}
		chunks = append(chunks, strings.Join(lines[start:end], "\n"))
		if end >= len(lines) {
			break
		}
		nextStart := end - overlapLines
		if nextStart <= start {
			nextStart = end
		}
		start = nextStart
	}
	return chunks
}

func countSummaryLines(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(text), "\n"))
}
