package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

func TestLogStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)

	conversation := "demo"
	systemPrompt := "Be terse."
	schemaJSON := `{"type":"object"}`
	fragmentsJSON := `["ctx","readme"]`
	reductionJSON := `{"mode":"summarize","summarized":true}`
	inputTokens := int64(3)
	outputTokens := int64(7)
	durationMS := int64(42)
	entry := model.LogEntry{
		Model:         "gpt-4o-mini",
		Provider:      "openai",
		Prompt:        "hello",
		Response:      "world",
		Conversation:  &conversation,
		SystemPrompt:  &systemPrompt,
		SchemaJSON:    &schemaJSON,
		FragmentsJSON: &fragmentsJSON,
		ReductionJSON: &reductionJSON,
		InputTokens:   &inputTokens,
		OutputTokens:  &outputTokens,
		DurationMS:    &durationMS,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}

	if err := store.Insert(entry); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	entries, err := store.Query(LogFilter{Model: "gpt-4o-mini", Limit: 10})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Prompt != "hello" || entries[0].Response != "world" {
		t.Fatalf("unexpected log entry: %+v", entries[0])
	}
	if entries[0].SchemaJSON == nil || *entries[0].SchemaJSON != schemaJSON || entries[0].FragmentsJSON == nil || *entries[0].FragmentsJSON != fragmentsJSON || entries[0].ReductionJSON == nil || *entries[0].ReductionJSON != reductionJSON {
		t.Fatalf("expected schema, fragments, and reduction in log entry: %+v", entries[0])
	}

	got, err := store.Get(entries[0].ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil || got.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected get result: %+v", got)
	}
	if got.SchemaJSON == nil || *got.SchemaJSON != schemaJSON || got.FragmentsJSON == nil || *got.FragmentsJSON != fragmentsJSON || got.ReductionJSON == nil || *got.ReductionJSON != reductionJSON {
		t.Fatalf("expected schema/fragments/reduction in get result: %+v", got)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count after clear failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count=0 after clear, got %d", count)
	}
}

func TestLogStoreMigratesReductionColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	_, err = db.Exec(`
CREATE TABLE logs (
    id TEXT PRIMARY KEY,
    conversation TEXT,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    system_prompt TEXT,
    schema_json TEXT,
    fragments_json TEXT,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    duration_ms INTEGER,
    created_at TEXT NOT NULL
);
CREATE VIRTUAL TABLE logs_fts USING fts4(id, prompt, response);`)
	if err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	store := NewLogStore(path)
	reductionJSON := `{"mode":"truncate"}`
	if err := store.Insert(model.LogEntry{
		ID:            "001",
		Model:         "gpt-4o-mini",
		Provider:      "openai",
		Prompt:        "hello",
		Response:      "world",
		ReductionJSON: &reductionJSON,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("insert failed after migration: %v", err)
	}

	entry, err := store.Get("001")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if entry == nil || entry.ReductionJSON == nil || *entry.ReductionJSON != reductionJSON {
		t.Fatalf("expected migrated reduction json, got %+v", entry)
	}
}

func TestLogStoreCreatesDatabaseWithSecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)

	if err := store.Insert(model.LogEntry{
		Model:    "gpt-4o-mini",
		Provider: "openai",
		Prompt:   "hello",
		Response: "world",
	}); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != secureFileMode {
		t.Fatalf("expected secure db mode, got %o", got)
	}
}

func TestLogStoreQueryFiltersAndMissingGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)
	conversation := "demo"
	now := time.Now().UTC()
	entries := []model.LogEntry{
		{Model: "gpt-4o-mini", Provider: "openai", Prompt: "hello", Response: "world", Conversation: &conversation, CreatedAt: now},
		{Model: "claude", Provider: "anthropic", Prompt: "other", Response: "reply", CreatedAt: now.Add(time.Second)},
	}
	for _, entry := range entries {
		if err := store.Insert(entry); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}
	filtered, err := store.Query(LogFilter{Conversation: "demo", Search: "wor", Limit: 1})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Model != "gpt-4o-mini" {
		t.Fatalf("unexpected filtered logs: %+v", filtered)
	}
	got, err := store.Get("missing")
	if err != nil {
		t.Fatalf("missing get failed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing log, got %+v", got)
	}
}

func TestLogStoreQueryLatestConversationAndConversationFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)
	alpha := "alpha"
	beta := "beta"
	entries := []model.LogEntry{
		{ID: "001", Model: "gpt-4o-mini", Provider: "openai", Prompt: "one", Response: "one", Conversation: &alpha, CreatedAt: time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)},
		{ID: "002", Model: "gpt-4o-mini", Provider: "openai", Prompt: "two", Response: "two", Conversation: &beta, CreatedAt: time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC)},
		{ID: "003", Model: "gpt-4o-mini", Provider: "openai", Prompt: "three", Response: "three", Conversation: &beta, CreatedAt: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)},
	}
	for _, entry := range entries {
		if err := store.Insert(entry); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	latestConversation, err := store.Query(LogFilter{LatestConversation: true, Limit: 10})
	if err != nil {
		t.Fatalf("latest conversation query failed: %v", err)
	}
	if len(latestConversation) != 2 || latestConversation[0].Conversation == nil || *latestConversation[0].Conversation != "beta" {
		t.Fatalf("unexpected latest conversation logs: %+v", latestConversation)
	}

	alphaLogs, err := store.Query(LogFilter{Conversation: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("conversation query failed: %v", err)
	}
	if len(alphaLogs) != 1 || alphaLogs[0].ID != "001" {
		t.Fatalf("unexpected alpha logs: %+v", alphaLogs)
	}
}

func TestLogStoreQuerySearchSortingAndIDFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)
	entries := []model.LogEntry{
		{ID: "001", Model: "gpt-4o-mini", Provider: "openai", Prompt: "cheese", Response: "plain", CreatedAt: time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)},
		{ID: "002", Model: "gpt-4o-mini", Provider: "openai", Prompt: "other", Response: "contains cheese here", CreatedAt: time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC)},
		{ID: "003", Model: "gpt-4o-mini", Provider: "openai", Prompt: "cheese in prompt", Response: "later", CreatedAt: time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)},
	}
	for _, entry := range entries {
		if err := store.Insert(entry); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	relevance, err := store.Query(LogFilter{Search: "cheese", Limit: 10})
	if err != nil {
		t.Fatalf("relevance query failed: %v", err)
	}
	if len(relevance) != 3 || relevance[0].ID != "001" {
		t.Fatalf("expected exact match first, got %+v", relevance)
	}

	latest, err := store.Query(LogFilter{Search: "cheese", Latest: true, Limit: 10})
	if err != nil {
		t.Fatalf("latest query failed: %v", err)
	}
	if len(latest) != 3 || latest[0].ID != "002" {
		t.Fatalf("expected latest-first ordering, got %+v", latest)
	}

	idGT, err := store.Query(LogFilter{IDGT: "001", Limit: 10})
	if err != nil {
		t.Fatalf("id-gt query failed: %v", err)
	}
	if len(idGT) != 2 || idGT[0].ID != "002" || idGT[1].ID != "003" {
		t.Fatalf("unexpected id-gt logs: %+v", idGT)
	}

	idGTE, err := store.Query(LogFilter{IDGTE: "002", Limit: 10})
	if err != nil {
		t.Fatalf("id-gte query failed: %v", err)
	}
	if len(idGTE) != 2 || idGTE[0].ID != "002" || idGTE[1].ID != "003" {
		t.Fatalf("unexpected id-gte logs: %+v", idGTE)
	}
}

func TestLogStoreMigratesOldSchemaAndBuildsFTS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	_, err = db.Exec(`
CREATE TABLE logs (
    id TEXT PRIMARY KEY,
    conversation TEXT,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    system_prompt TEXT,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    duration_ms INTEGER,
    created_at TEXT NOT NULL
)`)
	if err != nil {
		t.Fatalf("create legacy logs table failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO logs (id, conversation, model, provider, system_prompt, prompt, response, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"001", "demo", "gpt-4o-mini", "openai", "be terse", "legacy prompt", "legacy response", time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert legacy row failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	store := NewLogStore(path)
	results, err := store.Query(LogFilter{Search: "legacy", Limit: 10})
	if err != nil {
		t.Fatalf("query after migration failed: %v", err)
	}
	if len(results) != 1 || results[0].ID != "001" {
		t.Fatalf("unexpected migrated results: %+v", results)
	}
	got, err := store.Get("001")
	if err != nil {
		t.Fatalf("get after migration failed: %v", err)
	}
	if got == nil || got.SchemaJSON != nil || got.FragmentsJSON != nil {
		t.Fatalf("expected migrated row with nil new fields, got %+v", got)
	}
}

func TestLogStoreRepairsFTSOnlyWhenOutOfSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)
	entry := model.LogEntry{
		ID:        "001",
		Model:     "gpt-4o-mini",
		Provider:  "openai",
		Prompt:    "hello",
		Response:  "world",
		CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}
	if err := store.Insert(entry); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM logs_fts`); err != nil {
		t.Fatalf("delete logs_fts failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	repairStore := NewLogStore(path)
	results, err := repairStore.Query(LogFilter{Search: "hello", Limit: 10})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(results) != 1 || results[0].ID != "001" {
		t.Fatalf("expected repaired FTS entry, got %+v", results)
	}
}

func TestLogStoreBackupCreatesUsableCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	backup := filepath.Join(t.TempDir(), "backup", "logs-copy.db")
	store := NewLogStore(path)
	entry := model.LogEntry{
		ID:        "001",
		Model:     "gpt-4o-mini",
		Provider:  "openai",
		Prompt:    "hello",
		Response:  "world",
		CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}
	if err := store.Insert(entry); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if err := store.Backup(backup); err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	backupStore := NewLogStore(backup)
	results, err := backupStore.Query(LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("backup query failed: %v", err)
	}
	if len(results) != 1 || results[0].ID != "001" {
		t.Fatalf("unexpected backup contents: %+v", results)
	}
}
