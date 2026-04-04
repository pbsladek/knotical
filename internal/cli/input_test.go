package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditorCommandArgs(t *testing.T) {
	name, args, err := editorCommandArgs("code --wait", "/tmp/prompt.txt")
	if err != nil {
		t.Fatalf("editorCommandArgs failed: %v", err)
	}
	if name != "code" {
		t.Fatalf("unexpected command name: %q", name)
	}
	if len(args) != 2 || args[0] != "--wait" || args[1] != "/tmp/prompt.txt" {
		t.Fatalf("unexpected command args: %+v", args)
	}
}

func TestEditorCommandArgsRejectsEmptyCommand(t *testing.T) {
	if _, _, err := editorCommandArgs("   ", "/tmp/prompt.txt"); err == nil {
		t.Fatal("expected empty editor command error")
	}
}

func TestOpenEditorForPromptRunsEditorCommand(t *testing.T) {
	editorScript := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nfor last; do :; done\nprintf 'prompt from editor' > \"$last\"\n"), 0o755); err != nil {
		t.Fatalf("write editor script failed: %v", err)
	}
	t.Setenv("EDITOR", editorScript+" --wait")

	prompt, err := openEditorForPrompt()
	if err != nil {
		t.Fatalf("openEditorForPrompt failed: %v", err)
	}
	if prompt != "prompt from editor" {
		t.Fatalf("unexpected prompt: %q", prompt)
	}
}
