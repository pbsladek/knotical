package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pbsladek/knotical/internal/model"
)

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
