package store

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/pbsladek/knotical/internal/model"
)

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
