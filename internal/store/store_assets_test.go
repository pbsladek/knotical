package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

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
