package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/store"
)

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
