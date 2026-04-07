package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

type logsRenderOptions struct {
	JSON         bool
	ResponseOnly bool
	Extract      bool
	ExtractLast  bool
	Short        bool
}

func renderLogEntries(entries []model.LogEntry, opts logsRenderOptions) (string, error) {
	switch {
	case opts.JSON:
		return renderLogEntriesJSON(entries)
	case opts.ResponseOnly:
		return joinRendered(entries, func(entry model.LogEntry) string { return entry.Response }), nil
	case opts.Extract:
		return joinRendered(entries, func(entry model.LogEntry) string { return extractLogCodeBlock(entry.Response, false) }), nil
	case opts.ExtractLast:
		return joinRendered(entries, func(entry model.LogEntry) string { return extractLogCodeBlock(entry.Response, true) }), nil
	case opts.Short:
		return renderShortLogEntries(entries), nil
	default:
		return renderDefaultLogEntries(entries), nil
	}
}

func renderLogEntriesJSON(entries []model.LogEntry) (string, error) {
	payload, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}

func renderDefaultLogEntries(entries []model.LogEntry) string {
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("%s%s%s%s\n", "\033[1m", "\033[36m", fmt.Sprintf("%s %s %s", entry.ID, entry.Model, entry.CreatedAt.Format(time.RFC3339)), "\033[0m"))
		builder.WriteString(fmt.Sprintf("P: %.80s\n", entry.Prompt))
		builder.WriteString(fmt.Sprintf("R: %.80s\n\n", entry.Response))
	}
	return builder.String()
}

func renderShortLogEntries(entries []model.LogEntry) string {
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("- model: %s\n", entry.Model))
		builder.WriteString(fmt.Sprintf("  datetime: '%s'\n", entry.CreatedAt.Format(time.RFC3339)))
		if entry.Conversation != nil {
			builder.WriteString(fmt.Sprintf("  conversation: %s\n", *entry.Conversation))
		}
		if entry.SystemPrompt != nil && *entry.SystemPrompt != "" {
			builder.WriteString(fmt.Sprintf("  system: %s\n", truncateLogField(*entry.SystemPrompt)))
		}
		builder.WriteString(fmt.Sprintf("  prompt: %s\n", truncateLogField(entry.Prompt)))
	}
	return builder.String()
}

func truncateLogField(value string) string {
	const maxLen = 120
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func joinRendered(entries []model.LogEntry, render func(model.LogEntry) string) string {
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		rendered := render(entry)
		if rendered == "" {
			continue
		}
		values = append(values, rendered)
	}
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, "\n") + "\n"
}

func extractLogCodeBlock(text string, last bool) string {
	lines := strings.Split(text, "\n")
	blocks := []string{}
	inBlock := false
	current := []string{}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inBlock {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inBlock = false
				continue
			}
			inBlock = true
			current = []string{}
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	if last {
		return blocks[len(blocks)-1]
	}
	return blocks[0]
}

func formatReductionDetails(raw *string) string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return ""
	}
	meta, ok := parseReductionMetadata(*raw)
	if !ok {
		return ""
	}
	lines := []string{}
	appendReductionLine(&lines, meta.Mode, "mode")
	appendReductionLine(&lines, meta.Profile, "profile")
	appendReductionLine(&lines, meta.StdinLabel, "stdin_label")
	appendReductionList(&lines, "shorthands", meta.Shorthands)
	appendReductionList(&lines, "transforms", meta.Transforms)
	appendReductionRange(&lines, "bytes", meta.OriginalBytes, meta.FinalBytes)
	appendReductionRange(&lines, "lines", meta.OriginalLines, meta.FinalLines)
	appendReductionRange(&lines, "tokens", meta.OriginalTokens, meta.FinalTokens)
	appendReductionInt(&lines, "dropped_lines", meta.DroppedLines)
	appendReductionInt(&lines, "unique_groups", meta.UniqueGroups)
	if meta.Summarized {
		lines = append(lines, fmt.Sprintf("summarized: true (%d chunks)", meta.SummaryChunks))
	}
	appendReductionLine(&lines, meta.IntermediateModel, "intermediate_model")
	appendReductionList(&lines, "steps", meta.Steps)
	return strings.Join(lines, "\n")
}

func parseReductionMetadata(raw string) (model.ReductionMetadata, bool) {
	var meta model.ReductionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return model.ReductionMetadata{}, false
	}
	return meta, true
}

func appendReductionLine(lines *[]string, value string, label string) {
	if value != "" {
		*lines = append(*lines, fmt.Sprintf("%s: %s", label, value))
	}
}

func appendReductionList(lines *[]string, label string, values []string) {
	if len(values) > 0 {
		*lines = append(*lines, label+": "+strings.Join(values, ", "))
	}
}

func appendReductionRange(lines *[]string, label string, original int, final int) {
	if original > 0 || final > 0 {
		*lines = append(*lines, fmt.Sprintf("%s: %d -> %d", label, original, final))
	}
}

func appendReductionInt(lines *[]string, label string, value int) {
	if value > 0 {
		*lines = append(*lines, fmt.Sprintf("%s: %d", label, value))
	}
}
