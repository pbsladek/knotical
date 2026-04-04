package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/store"
)

func setupCLIConfigHome(t *testing.T) {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
}

func TestAliasesCommandSetListRemove(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	cmd := newAliasesCommand()
	cmd.SetArgs([]string{"set", "fast", "gpt-4o-mini"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("aliases set failed: %v", err)
	}

	cmd = newAliasesCommand()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("aliases list failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "fast\tgpt-4o-mini") {
		t.Fatalf("expected alias list output, got %q", outputBuffer.String())
	}

	cmd = newAliasesCommand()
	cmd.SetArgs([]string{"remove", "fast"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("aliases remove failed: %v", err)
	}
	values, err := (store.JSONMapStore{Path: config.AliasesFilePath()}).Load()
	if err != nil {
		t.Fatalf("alias load failed: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected empty aliases after remove, got %+v", values)
	}
}

func TestChatsCommandListShowDelete(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	chatStore := store.ChatStore{Dir: config.ChatCacheDir()}
	session := model.NewChatSession("demo")
	session.PushUser("hello")
	if err := chatStore.Save(session); err != nil {
		t.Fatalf("chat save failed: %v", err)
	}

	cmd := newChatsCommand()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chats list failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "demo") {
		t.Fatalf("expected chat list output, got %q", outputBuffer.String())
	}

	outputBuffer.Reset()
	cmd = newChatsCommand()
	cmd.SetArgs([]string{"show", "demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chats show failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "[user] hello") {
		t.Fatalf("expected chat show output, got %q", outputBuffer.String())
	}

	cmd = newChatsCommand()
	cmd.SetArgs([]string{"delete", "demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chats delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.ChatCacheDir(), "demo.json")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted chat file, err=%v", err)
	}
}

func TestFragmentsCommandSetGetDelete(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	cmd := newFragmentsCommand()
	cmd.SetArgs([]string{"set", "ctx", "fragment body"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fragments set failed: %v", err)
	}

	cmd = newFragmentsCommand()
	cmd.SetArgs([]string{"get", "ctx"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fragments get failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "fragment body") {
		t.Fatalf("expected fragment output, got %q", outputBuffer.String())
	}

	cmd = newFragmentsCommand()
	cmd.SetArgs([]string{"delete", "ctx"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fragments delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.FragmentsDir(), "ctx.md")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted fragment file, err=%v", err)
	}
}

func TestKeysCommandGetListRemoveAndPath(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	manager := store.NewKeyManager(config.KeysFilePath())
	if err := manager.Set("openai", "sk-test-1234567890"); err != nil {
		t.Fatalf("key set failed: %v", err)
	}

	cmd := newKeysCommand()
	cmd.SetArgs([]string{"get", "openai"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("keys get failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "sk-t") {
		t.Fatalf("expected masked key output, got %q", outputBuffer.String())
	}

	outputBuffer.Reset()
	cmd = newKeysCommand()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("keys list failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "openai") {
		t.Fatalf("expected keys list output, got %q", outputBuffer.String())
	}

	outputBuffer.Reset()
	cmd = newKeysCommand()
	cmd.SetArgs([]string{"path"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("keys path failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), config.KeysFilePath()) {
		t.Fatalf("expected keys path output, got %q", outputBuffer.String())
	}

	cmd = newKeysCommand()
	cmd.SetArgs([]string{"remove", "openai"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("keys remove failed: %v", err)
	}
	if _, ok, err := manager.Get("openai"); err != nil || ok {
		t.Fatalf("expected removed key, ok=%v err=%v", ok, err)
	}
}

func TestRolesCommandCreateShowDelete(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	cmd := newRolesCommand()
	cmd.SetArgs([]string{"create", "--system", "Be terse", "reviewer"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roles create failed: %v", err)
	}

	cmd = newRolesCommand()
	cmd.SetArgs([]string{"show", "reviewer"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roles show failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "Be terse") {
		t.Fatalf("expected role show output, got %q", outputBuffer.String())
	}

	outputBuffer.Reset()
	cmd = newRolesCommand()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roles list failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "reviewer") {
		t.Fatalf("expected roles list output, got %q", outputBuffer.String())
	}

	cmd = newRolesCommand()
	cmd.SetArgs([]string{"delete", "reviewer"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roles delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.RolesDir(), "reviewer.toml")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted role file, err=%v", err)
	}
}

func TestRolesCommandCreateUsesEditor(t *testing.T) {
	setupCLIConfigHome(t)

	editorScript := filepath.Join(t.TempDir(), "role-editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nfor last; do :; done\nprintf 'Prompt from editor' > \"$last\"\n"), 0o755); err != nil {
		t.Fatalf("write editor script failed: %v", err)
	}
	t.Setenv("EDITOR", editorScript+" --wait")

	cmd := newRolesCommand()
	cmd.SetArgs([]string{"create", "editor-role"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roles create via editor failed: %v", err)
	}

	role, err := (store.RoleStore{Dir: config.RolesDir()}).Load("editor-role")
	if err != nil {
		t.Fatalf("role load failed: %v", err)
	}
	if role.SystemPrompt != "Prompt from editor" {
		t.Fatalf("unexpected role prompt: %+v", role)
	}
}

func TestTemplatesCommandCreateShowDelete(t *testing.T) {
	setupCLIConfigHome(t)
	outputBuffer := captureDefaultOutput(t)

	cmd := newTemplatesCommand()
	cmd.SetArgs([]string{"create", "review", "--system", "Be terse", "--model", "gpt-4o-mini", "--description", "Review template"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates create failed: %v", err)
	}

	cmd = newTemplatesCommand()
	cmd.SetArgs([]string{"show", "review"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates show failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "gpt-4o-mini") || !strings.Contains(outputBuffer.String(), "Be terse") {
		t.Fatalf("expected template show output, got %q", outputBuffer.String())
	}

	outputBuffer.Reset()
	cmd = newTemplatesCommand()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates list failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "review") {
		t.Fatalf("expected template list output, got %q", outputBuffer.String())
	}

	cmd = newTemplatesCommand()
	cmd.SetArgs([]string{"delete", "review"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.TemplatesDir(), "review.toml")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted template file, err=%v", err)
	}
}

func TestTemplatesCommandEditUsesEditorCommandArgs(t *testing.T) {
	setupCLIConfigHome(t)

	editorScript := filepath.Join(t.TempDir(), "template-editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nfor last; do :; done\ncat <<'EOF' > \"$last\"\nsystem_prompt = \"Edited prompt\"\nmodel = \"gpt-4o-mini\"\nEOF\n"), 0o755); err != nil {
		t.Fatalf("write editor script failed: %v", err)
	}
	t.Setenv("EDITOR", editorScript+" --wait")

	cmd := newTemplatesCommand()
	cmd.SetArgs([]string{"edit", "edited"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates edit failed: %v", err)
	}

	template, err := (store.TemplateStore{Dir: config.TemplatesDir()}).Load("edited")
	if err != nil {
		t.Fatalf("template load failed: %v", err)
	}
	if template.SystemPrompt != "Edited prompt" || template.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected edited template: %+v", template)
	}
}

func TestTemplatesCommandEditRejectsTraversalName(t *testing.T) {
	setupCLIConfigHome(t)

	cmd := newTemplatesCommand()
	cmd.SetArgs([]string{"edit", "../escape"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected traversal template name to fail")
	}
}

func TestInstallIntegrationCommandInstallsAndWarns(t *testing.T) {
	setupCLIConfigHome(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir failed: %v", err)
	}
	t.Setenv("SHELL", "/bin/zsh")
	outputBuffer := captureDefaultOutput(t)

	cmd := newInstallIntegrationCommand()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install integration failed: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("read zshrc failed: %v", err)
	}
	if !strings.Contains(string(payload), "_knotical_widget") {
		t.Fatalf("expected zsh integration snippet, got %q", string(payload))
	}
	binaryPath, err := shellIntegrationBinary()
	if err != nil {
		t.Fatalf("shellIntegrationBinary failed: %v", err)
	}
	if !strings.Contains(string(payload), shellSingleQuote(binaryPath)) {
		t.Fatalf("expected quoted binary path in snippet, got %q", string(payload))
	}
	info, err := os.Stat(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("stat zshrc failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected secure rc file mode, got %o", got)
	}

	cmd = newInstallIntegrationCommand()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second install integration failed: %v", err)
	}
	if !strings.Contains(outputBuffer.String(), "already installed") {
		t.Fatalf("expected already-installed warning, got %q", outputBuffer.String())
	}
}

func TestShellSingleQuote(t *testing.T) {
	got := shellSingleQuote("/tmp/it's knotical")
	if got != `'/tmp/it'\''s knotical'` {
		t.Fatalf("unexpected quoted value: %q", got)
	}
}
