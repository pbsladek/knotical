package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/store"
)

func TestRenderLogEntriesJSON(t *testing.T) {
	rendered, err := renderLogEntries(sampleLogEntries(), logsRenderOptions{JSON: true})
	if err != nil {
		t.Fatalf("renderLogEntries failed: %v", err)
	}
	if !strings.Contains(rendered, `"model": "gpt-4o-mini"`) {
		t.Fatalf("expected JSON output, got %q", rendered)
	}
}

func TestRenderLogEntriesResponseOnly(t *testing.T) {
	rendered, err := renderLogEntries(sampleLogEntries(), logsRenderOptions{ResponseOnly: true})
	if err != nil {
		t.Fatalf("renderLogEntries failed: %v", err)
	}
	if rendered != "first response\nsecond response\n" {
		t.Fatalf("unexpected response output: %q", rendered)
	}
}

func TestRenderLogEntriesExtractFirstAndLast(t *testing.T) {
	entries := []model.LogEntry{{
		Response: "before\n```go\nfmt.Println(\"first\")\n```\nmiddle\n```go\nfmt.Println(\"last\")\n```\nafter",
	}}
	first, err := renderLogEntries(entries, logsRenderOptions{Extract: true})
	if err != nil {
		t.Fatalf("extract first failed: %v", err)
	}
	last, err := renderLogEntries(entries, logsRenderOptions{ExtractLast: true})
	if err != nil {
		t.Fatalf("extract last failed: %v", err)
	}
	if first != "fmt.Println(\"first\")\n" {
		t.Fatalf("unexpected first block: %q", first)
	}
	if last != "fmt.Println(\"last\")\n" {
		t.Fatalf("unexpected last block: %q", last)
	}
}

func TestRenderLogEntriesShort(t *testing.T) {
	rendered, err := renderLogEntries(sampleLogEntries(), logsRenderOptions{Short: true})
	if err != nil {
		t.Fatalf("render short failed: %v", err)
	}
	if !strings.Contains(rendered, "- model: gpt-4o-mini") {
		t.Fatalf("expected short model line, got %q", rendered)
	}
	if !strings.Contains(rendered, "conversation: demo") {
		t.Fatalf("expected conversation in short output, got %q", rendered)
	}
}

func TestExtractLogCodeBlockMissing(t *testing.T) {
	if got := extractLogCodeBlock("plain text", false); got != "" {
		t.Fatalf("expected empty extract, got %q", got)
	}
}

func TestBuildLogFilter(t *testing.T) {
	filter, err := buildLogFilter(logsQueryOptions{
		Count:              5,
		Model:              "gpt-4o-mini",
		Search:             "cheese",
		LatestConversation: true,
		Latest:             true,
		IDGT:               "001",
	})
	if err != nil {
		t.Fatalf("buildLogFilter failed: %v", err)
	}
	if !filter.LatestConversation || !filter.Latest || filter.IDGT != "001" || filter.Model != "gpt-4o-mini" || filter.Search != "cheese" || filter.Limit != 5 {
		t.Fatalf("unexpected filter: %+v", filter)
	}
}

func TestBuildLogFilterRejectsConflicts(t *testing.T) {
	if _, err := buildLogFilter(logsQueryOptions{LatestConversation: true, Conversation: "demo"}); err == nil {
		t.Fatal("expected conversation conflict")
	}
	if _, err := buildLogFilter(logsQueryOptions{IDGT: "001", IDGTE: "002"}); err == nil {
		t.Fatal("expected id filter conflict")
	}
}

func TestResolveLogsBackupPath(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	got, err := resolveLogsBackupPath(nil)
	if err != nil {
		t.Fatalf("resolveLogsBackupPath failed: %v", err)
	}
	if !strings.Contains(got, "logs-backup-") || !strings.HasSuffix(got, ".db") {
		t.Fatalf("unexpected generated backup path: %q", got)
	}

	explicit, err := resolveLogsBackupPath([]string{"/tmp/custom.db"})
	if err != nil {
		t.Fatalf("resolveLogsBackupPath explicit failed: %v", err)
	}
	if explicit != "/tmp/custom.db" {
		t.Fatalf("unexpected explicit path: %q", explicit)
	}
}

func TestBackupLogsDatabase(t *testing.T) {
	source := filepath.Join(t.TempDir(), "logs.db")
	destination := filepath.Join(t.TempDir(), "backup", "logs-copy.db")
	logStore := store.NewLogStore(source)
	entry := model.LogEntry{
		ID:        "001",
		Model:     "gpt-4o-mini",
		Provider:  "openai",
		Prompt:    "hello",
		Response:  "world",
		CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}
	if err := logStore.Insert(entry); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := backupLogsDatabase(logStore, destination); err != nil {
		t.Fatalf("backupLogsDatabase failed: %v", err)
	}

	backupStore := store.NewLogStore(destination)
	entries, err := backupStore.Query(store.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("query destination failed: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "001" {
		t.Fatalf("unexpected backup content: %+v", entries)
	}
}

func sampleLogEntries() []model.LogEntry {
	conversation := "demo"
	system := "be terse"
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	return []model.LogEntry{
		{
			ID:           "log-1",
			Model:        "gpt-4o-mini",
			Provider:     "openai",
			Prompt:       "first prompt",
			Response:     "first response",
			Conversation: &conversation,
			SystemPrompt: &system,
			CreatedAt:    now,
		},
		{
			ID:        "log-2",
			Model:     "claude-sonnet-4-5",
			Provider:  "anthropic",
			Prompt:    "second prompt",
			Response:  "second response",
			CreatedAt: now.Add(time.Minute),
		},
	}
}
