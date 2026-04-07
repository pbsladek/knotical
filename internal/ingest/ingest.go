package ingest

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/pbsladek/knotical/internal/model"
)

const (
	ModeAuto    = "auto"
	ModeAppend  = "append"
	ModeReplace = "replace"

	ReductionOff       = "off"
	ReductionTruncate  = "truncate"
	ReductionFail      = "fail"
	ReductionSummarize = "summarize"
)

type Options struct {
	InstructionText string
	StdinText       string
	StdinMode       string
	StdinLabel      string
	Profile         string
	Shorthands      []string
	Transforms      []string
	NoPipeline      bool
	MaxInputBytes   int
	MaxInputLines   int
	MaxInputTokens  int
	InputReduction  string
	HeadLines       int
	TailLines       int
	SampleLines     int
}

type Result struct {
	PromptText         string
	InputText          string
	NeedsSummarization bool
	Reduction          *model.ReductionMetadata
}

func Process(opts Options) (Result, error) {
	mode := normalizeMode(opts.StdinMode)
	stdinText := strings.TrimSpace(opts.StdinText)
	instructionText := strings.TrimSpace(opts.InstructionText)
	if stdinText == "" {
		return processWithoutStdin(mode, instructionText)
	}

	label := strings.TrimSpace(opts.StdinLabel)
	if label == "" {
		label = "input"
	}
	report := newReductionMetadata(label, opts.InputReduction, stdinText)
	pipeline, err := resolveInputPipeline(opts)
	if err != nil {
		return Result{}, err
	}
	opts = applyResolvedPipelineDefaults(opts, pipeline, report)
	reduced, summarize, err := processStdinText(stdinText, opts, pipeline, report)
	if err != nil {
		return Result{}, err
	}
	updateReductionFinal(report, reduced)
	return composeInputResult(mode, instructionText, label, reduced, summarize, report), nil
}

func processWithoutStdin(mode string, instructionText string) (Result, error) {
	if mode == ModeReplace {
		return Result{}, fmt.Errorf("--stdin-mode replace requires piped stdin")
	}
	if instructionText == "" {
		return Result{}, fmt.Errorf("no prompt provided")
	}
	return Result{PromptText: instructionText}, nil
}

func newReductionMetadata(label string, inputReduction string, stdinText string) *model.ReductionMetadata {
	return &model.ReductionMetadata{
		StdinLabel:     label,
		Mode:           normalizeReductionMode(inputReduction),
		OriginalBytes:  len(stdinText),
		OriginalLines:  countLines(stdinText),
		OriginalTokens: EstimateTokens(stdinText),
	}
}

func resolveInputPipeline(opts Options) (Pipeline, error) {
	return ResolvePipeline(PipelineOptions{
		Profile:    opts.Profile,
		Shorthands: opts.Shorthands,
		Transforms: opts.Transforms,
		NoPipeline: opts.NoPipeline,
	})
}

func applyResolvedPipelineDefaults(opts Options, pipeline Pipeline, report *model.ReductionMetadata) Options {
	if pipeline.Profile.Name != "" || len(pipeline.Transforms) > 0 || len(pipeline.Shorthands) > 0 {
		opts = applyPipelineDefaults(opts, pipeline)
		report.Profile = pipeline.Profile.Name
		report.Shorthands = append([]string(nil), pipeline.Shorthands...)
		report.Transforms = transformSpecNames(pipeline.Transforms)
	}
	return opts
}

func processStdinText(stdinText string, opts Options, pipeline Pipeline, report *model.ReductionMetadata) (string, bool, error) {
	if opts.MaxInputBytes > 0 {
		stdinText = truncateUTF8Bytes(stdinText, opts.MaxInputBytes)
		report.Steps = append(report.Steps, fmt.Sprintf("max-bytes:%d", opts.MaxInputBytes))
	}
	if pipeline.Profile.Name != "" || len(pipeline.Transforms) > 0 {
		var err error
		stdinText, err = ApplyPipeline(stdinText, pipeline, report)
		if err != nil {
			return "", false, err
		}
	}
	workingOpts := opts
	workingOpts.MaxInputBytes = 0
	reduced := reduceInput(stdinText, workingOpts, report)
	if reduced == "" {
		return "", false, fmt.Errorf("empty prompt from stdin")
	}
	return applyTokenBudget(reduced, opts, report)
}

func composeInputResult(mode string, instructionText string, label string, reduced string, summarize bool, report *model.ReductionMetadata) Result {
	result := Result{
		InputText:          reduced,
		NeedsSummarization: summarize,
		Reduction:          report,
	}
	if mode == ModeReplace || instructionText == "" {
		result.PromptText = reduced
		return result
	}
	result.PromptText = instructionText + "\n\n" + label + ":\n" + reduced
	return result
}

func normalizeMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ModeAppend:
		return ModeAppend
	case ModeReplace:
		return ModeReplace
	default:
		return ModeAuto
	}
}

func reduceInput(text string, opts Options, report *model.ReductionMetadata) string {
	lines := splitLines(text)
	originalLen := len(lines)
	lines = selectHeadTail(lines, opts.HeadLines, opts.TailLines)
	if len(lines) != originalLen && (opts.HeadLines > 0 || opts.TailLines > 0) {
		report.Steps = append(report.Steps, fmt.Sprintf("head-tail:%d:%d", opts.HeadLines, opts.TailLines))
	}
	beforeSample := len(lines)
	lines = sampleDeterministic(lines, opts.SampleLines)
	if len(lines) != beforeSample && opts.SampleLines > 0 {
		report.Steps = append(report.Steps, fmt.Sprintf("sample:%d", opts.SampleLines))
	}
	if opts.MaxInputLines > 0 && len(lines) > opts.MaxInputLines {
		lines = lines[:opts.MaxInputLines]
		report.Steps = append(report.Steps, fmt.Sprintf("max-lines:%d", opts.MaxInputLines))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

func applyTokenBudget(text string, opts Options, report *model.ReductionMetadata) (string, bool, error) {
	if opts.MaxInputTokens <= 0 {
		return text, false, nil
	}
	estimate := EstimateTokens(text)
	if estimate <= opts.MaxInputTokens {
		return text, false, nil
	}
	switch normalizeReductionMode(opts.InputReduction) {
	case ReductionOff:
		report.Steps = append(report.Steps, fmt.Sprintf("max-tokens-off:%d", opts.MaxInputTokens))
		return text, false, nil
	case ReductionFail:
		return "", false, fmt.Errorf("input exceeds max token budget: estimated %d > %d", estimate, opts.MaxInputTokens)
	case ReductionSummarize:
		report.Steps = append(report.Steps, fmt.Sprintf("max-tokens-summarize:%d", opts.MaxInputTokens))
		return text, true, nil
	default:
		truncated := truncateToTokenBudget(text, opts.MaxInputTokens)
		if truncated == "" {
			return "", false, fmt.Errorf("input became empty after token truncation")
		}
		report.Steps = append(report.Steps, fmt.Sprintf("max-tokens-truncate:%d", opts.MaxInputTokens))
		return truncated, false, nil
	}
}

func normalizeReductionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ReductionOff:
		return ReductionOff
	case ReductionFail:
		return ReductionFail
	case ReductionSummarize:
		return ReductionSummarize
	default:
		return ReductionTruncate
	}
}

func TruncateToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	truncated := strings.TrimSpace(truncateUTF8Bytes(text, maxTokens*4))
	for truncated != "" && EstimateTokens(truncated) > maxTokens {
		runes := []rune(truncated)
		if len(runes) == 0 {
			return ""
		}
		truncated = strings.TrimSpace(string(runes[:len(runes)-1]))
	}
	return truncated
}

func truncateToTokenBudget(text string, maxTokens int) string {
	return TruncateToTokenBudget(text, maxTokens)
}

func truncateUTF8Bytes(text string, maxBytes int) string {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	size := 0
	for idx, r := range text {
		runeBytes := utf8.RuneLen(r)
		if runeBytes < 0 {
			runeBytes = 1
		}
		if size+runeBytes > maxBytes {
			return text[:idx]
		}
		size += runeBytes
	}
	return text
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func selectHeadTail(lines []string, head int, tail int) []string {
	if len(lines) == 0 || (head <= 0 && tail <= 0) {
		return lines
	}
	limitHead := minPositive(head, len(lines))
	limitTail := minPositive(tail, len(lines))
	indices := make([]int, 0, limitHead+limitTail)
	for idx := 0; idx < limitHead; idx++ {
		indices = append(indices, idx)
	}
	for idx := len(lines) - limitTail; idx < len(lines); idx++ {
		if idx >= 0 {
			indices = append(indices, idx)
		}
	}
	slices.Sort(indices)
	indices = slices.Compact(indices)
	selected := make([]string, 0, len(indices))
	for _, idx := range indices {
		selected = append(selected, lines[idx])
	}
	return selected
}

func sampleDeterministic(lines []string, sample int) []string {
	if sample <= 0 || len(lines) <= sample {
		return lines
	}
	if sample == 1 {
		return []string{lines[0]}
	}
	indices := make([]int, 0, sample)
	last := len(lines) - 1
	for i := 0; i < sample; i++ {
		idx := i * last / (sample - 1)
		indices = append(indices, idx)
	}
	slices.Sort(indices)
	indices = slices.Compact(indices)
	selected := make([]string, 0, len(indices))
	for _, idx := range indices {
		selected = append(selected, lines[idx])
	}
	return selected
}

func minPositive(value int, limit int) int {
	if value <= 0 {
		return 0
	}
	if value > limit {
		return limit
	}
	return value
}

func countLines(text string) int {
	if text == "" {
		return 0
	}
	return len(splitLines(text))
}

func updateReductionFinal(report *model.ReductionMetadata, text string) {
	if report == nil {
		return
	}
	report.FinalBytes = len(text)
	report.FinalLines = countLines(text)
	report.FinalTokens = EstimateTokens(text)
}
