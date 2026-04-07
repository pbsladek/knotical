package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

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
