package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/pbsladek/knotical/internal/model"
)

func TestFragmentStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := FragmentStore{Dir: dir}

	if err := store.Save("readme", "# Project\nHello"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	fragment, err := store.Load("readme")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if fragment.Name != "readme" || fragment.Content != "# Project\nHello" {
		t.Fatalf("unexpected fragment: %+v", fragment)
	}
}

func TestStoresRejectPathTraversalNames(t *testing.T) {
	dir := t.TempDir()

	if _, err := (ChatStore{Dir: dir}).LoadOrCreate("../escape"); err == nil {
		t.Fatal("expected chat store to reject traversal name")
	}
	if err := (TemplateStore{Dir: dir}).Save(Template{Name: "../escape"}); err == nil {
		t.Fatal("expected template store to reject traversal name")
	}
	if err := (RoleStore{Dir: dir}).Save(Role{Name: "../escape", SystemPrompt: "x"}); err == nil {
		t.Fatal("expected role store to reject traversal name")
	}
	if err := (FragmentStore{Dir: dir}).Save("../escape", "x"); err == nil {
		t.Fatal("expected fragment store to reject traversal name")
	}
}

func TestTemplateStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := TemplateStore{Dir: dir}
	temperature := 0.2

	template := Template{Name: "review", Model: "gpt-4o-mini", SystemPrompt: "Be terse.", Temperature: &temperature}
	if err := store.Save(template); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load("review")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Model != template.Model || loaded.SystemPrompt != template.SystemPrompt {
		t.Fatalf("unexpected template: %+v", loaded)
	}
}

func TestStoredFilesUseSecurePermissions(t *testing.T) {
	dir := t.TempDir()
	templateStore := TemplateStore{Dir: filepath.Join(dir, "templates")}
	roleStore := RoleStore{Dir: filepath.Join(dir, "roles")}
	fragmentStore := FragmentStore{Dir: filepath.Join(dir, "fragments")}
	chatStore := ChatStore{Dir: filepath.Join(dir, "chats")}

	if err := templateStore.Save(Template{Name: "review"}); err != nil {
		t.Fatalf("template save failed: %v", err)
	}
	if err := roleStore.Save(Role{Name: "reviewer", SystemPrompt: "be terse"}); err != nil {
		t.Fatalf("role save failed: %v", err)
	}
	if err := fragmentStore.Save("ctx", "hello"); err != nil {
		t.Fatalf("fragment save failed: %v", err)
	}
	session := model.NewChatSession("demo")
	if err := chatStore.Save(session); err != nil {
		t.Fatalf("chat save failed: %v", err)
	}

	for _, path := range []string{
		filepath.Join(templateStore.Dir, "review.toml"),
		filepath.Join(roleStore.Dir, "reviewer.toml"),
		filepath.Join(fragmentStore.Dir, "ctx.md"),
		filepath.Join(chatStore.Dir, "demo.json"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s failed: %v", path, err)
		}
		if got := info.Mode().Perm(); got != secureFileMode {
			t.Fatalf("expected secure file mode for %s, got %o", path, got)
		}
	}
}

func TestRoleStoreIncludesBuiltins(t *testing.T) {
	store := RoleStore{Dir: t.TempDir()}
	roles, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(roles) < 3 {
		t.Fatalf("expected builtin roles, got %d", len(roles))
	}
}

func TestJSONMapStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aliases.json")
	store := JSONMapStore{Path: path}
	values := map[string]string{"fast": "gpt-4o-mini"}
	if err := store.Save(values); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded["fast"] != "gpt-4o-mini" {
		t.Fatalf("unexpected loaded values: %+v", loaded)
	}
}

func TestTemplateAndFragmentListAndExists(t *testing.T) {
	dir := t.TempDir()
	templateStore := TemplateStore{Dir: filepath.Join(dir, "templates")}
	fragmentStore := FragmentStore{Dir: filepath.Join(dir, "fragments")}
	if err := templateStore.Save(Template{Name: "review"}); err != nil {
		t.Fatalf("template save failed: %v", err)
	}
	if err := fragmentStore.Save("ctx", "hello"); err != nil {
		t.Fatalf("fragment save failed: %v", err)
	}
	if !templateStore.Exists("review") {
		t.Fatalf("expected template to exist")
	}
	if !fragmentStore.Exists("ctx") {
		t.Fatalf("expected fragment to exist")
	}
	templates, err := templateStore.List()
	if err != nil {
		t.Fatalf("template list failed: %v", err)
	}
	fragments, err := fragmentStore.List()
	if err != nil {
		t.Fatalf("fragment list failed: %v", err)
	}
	if len(templates) != 1 || templates[0].Name != "review" {
		t.Fatalf("unexpected templates: %+v", templates)
	}
	if len(fragments) != 1 || fragments[0].Name != "ctx" {
		t.Fatalf("unexpected fragments: %+v", fragments)
	}
}

func TestCacheStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := CacheStore{Dir: dir}
	messages := []model.Message{{Role: model.RoleUser, Content: "hello"}}
	temperature := 0.3

	if err := store.Set("gpt-4o-mini", "be terse", messages, nil, &temperature, nil, "world"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, ok, err := store.Get("gpt-4o-mini", "be terse", messages, nil, &temperature, nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok || value != "world" {
		t.Fatalf("unexpected cache result: ok=%v value=%q", ok, value)
	}
}

func TestCacheStoreSeparatesSystemPrompts(t *testing.T) {
	dir := t.TempDir()
	store := CacheStore{Dir: dir}
	messages := []model.Message{{Role: model.RoleUser, Content: "hello"}}

	if err := store.Set("gpt-4o-mini", "be terse", messages, nil, nil, nil, "world"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if _, ok, err := store.Get("gpt-4o-mini", "be verbose", messages, nil, nil, nil); err != nil {
		t.Fatalf("Get failed: %v", err)
	} else if ok {
		t.Fatalf("expected cache miss for different system prompt")
	}
}

func TestCacheStoreSeparatesSchemas(t *testing.T) {
	dir := t.TempDir()
	store := CacheStore{Dir: dir}
	messages := []model.Message{{Role: model.RoleUser, Content: "hello"}}
	schemaA := map[string]any{"type": "object", "required": []string{"name"}}
	schemaB := map[string]any{"type": "object", "required": []string{"age"}}

	if err := store.Set("gpt-4o-mini", "be terse", messages, schemaA, nil, nil, "world"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if _, ok, err := store.Get("gpt-4o-mini", "be terse", messages, schemaB, nil, nil); err != nil {
		t.Fatalf("Get failed: %v", err)
	} else if ok {
		t.Fatal("expected cache miss for different schema")
	}
}

func TestChatStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := ChatStore{Dir: dir}
	session := model.NewChatSession("demo")
	session.PushUser("hello")

	if err := store.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.LoadOrCreate("demo")
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}
	if len(loaded.Messages) != 1 || loaded.Messages[0].Content != "hello" {
		t.Fatalf("unexpected chat session: %+v", loaded)
	}

	if _, err := filepath.Abs(dir); err != nil {
		t.Fatalf("Temp dir invalid: %v", err)
	}
}

func TestLogStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store := NewLogStore(path)

	conversation := "demo"
	systemPrompt := "Be terse."
	schemaJSON := `{"type":"object"}`
	fragmentsJSON := `["ctx","readme"]`
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
	if entries[0].SchemaJSON == nil || *entries[0].SchemaJSON != schemaJSON || entries[0].FragmentsJSON == nil || *entries[0].FragmentsJSON != fragmentsJSON {
		t.Fatalf("expected schema and fragments in log entry: %+v", entries[0])
	}

	got, err := store.Get(entries[0].ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil || got.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected get result: %+v", got)
	}
	if got.SchemaJSON == nil || *got.SchemaJSON != schemaJSON || got.FragmentsJSON == nil || *got.FragmentsJSON != fragmentsJSON {
		t.Fatalf("expected schema/fragments in get result: %+v", got)
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

func TestChatStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := ChatStore{Dir: dir}
	session := model.NewChatSession("demo")
	if err := store.Save(session); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	removed, err := store.Delete("demo")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !removed {
		t.Fatalf("expected remove=true")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo.json")); !os.IsNotExist(err) {
		t.Fatalf("expected session file removed, err=%v", err)
	}
}

func TestChatStoreListSorted(t *testing.T) {
	dir := t.TempDir()
	store := ChatStore{Dir: dir}
	for _, name := range []string{"zeta", "alpha"} {
		if err := store.Save(model.NewChatSession(name)); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}
	names, err := store.List()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !slices.Equal(names, []string{"alpha", "zeta"}) {
		t.Fatalf("unexpected names: %+v", names)
	}
}

func TestRoleStoreCustomOverrideAndDelete(t *testing.T) {
	dir := t.TempDir()
	store := RoleStore{Dir: dir}
	role := Role{Name: "default", SystemPrompt: "custom default", Description: "custom"}
	if err := store.Save(role); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded, err := store.Load("default")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.SystemPrompt != "custom default" {
		t.Fatalf("expected custom role override, got %+v", loaded)
	}
	if err := store.Delete("default"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	loaded, err = store.Load("default")
	if err != nil {
		t.Fatalf("builtin fallback load failed: %v", err)
	}
	if loaded.SystemPrompt == "custom default" {
		t.Fatalf("expected builtin role after delete, got %+v", loaded)
	}
}

func TestFragmentStoreDeleteAndMissing(t *testing.T) {
	dir := t.TempDir()
	store := FragmentStore{Dir: dir}
	if err := store.Save("ctx", "hello"); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if err := store.Delete("ctx"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if store.Exists("ctx") {
		t.Fatalf("expected fragment to be deleted")
	}
	if _, err := store.Load("ctx"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing fragment error, got %v", err)
	}
}

func TestTemplateStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := TemplateStore{Dir: dir}
	if err := store.Save(Template{Name: "review"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if err := store.Delete("review"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if store.Exists("review") {
		t.Fatalf("expected template to be deleted")
	}
}

func TestJSONMapStoreMissingFile(t *testing.T) {
	store := JSONMapStore{Path: filepath.Join(t.TempDir(), "aliases.json")}
	values, err := store.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected empty map, got %+v", values)
	}
}

func TestKeyManagerRoundTrip(t *testing.T) {
	manager := NewKeyManager(filepath.Join(t.TempDir(), "keys.json"))
	const providerName = "unit-test-provider"
	if err := manager.Set(providerName, "sk-test-123456"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	key, ok, err := manager.Get(providerName)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !ok || key != "sk-test-123456" {
		t.Fatalf("unexpected key lookup: ok=%v key=%q", ok, key)
	}
	required, err := manager.Require(providerName)
	if err != nil {
		t.Fatalf("require failed: %v", err)
	}
	if required != key {
		t.Fatalf("expected required key to match, got %q", required)
	}
	providers, err := manager.ListStored()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !slices.Equal(providers, []string{providerName}) {
		t.Fatalf("unexpected providers: %+v", providers)
	}
	if MaskKey(key) == key {
		t.Fatalf("expected masked key")
	}
	removed, err := manager.Remove(providerName)
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if !removed {
		t.Fatalf("expected remove=true")
	}
	if _, err := manager.Require(providerName); err == nil {
		t.Fatalf("expected missing key error")
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
