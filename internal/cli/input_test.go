package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func withTestStdin(t *testing.T, content string) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdin-*.txt")
	if err != nil {
		t.Fatalf("create temp stdin failed: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write temp stdin failed: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("seek temp stdin failed: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = file.Close()
	})
}

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

func TestReadPromptUsesPositionalPromptWithoutStdin(t *testing.T) {
	got, err := readPromptSource(rootOptions{Prompt: []string{"analyze", "these", "logs"}})
	if err != nil {
		t.Fatalf("readPromptSource failed: %v", err)
	}
	if got.instructionText != "analyze these logs" || got.stdinText != "" {
		t.Fatalf("unexpected prompt source: %+v", got)
	}
}

func TestReadPromptUsesStdinOnly(t *testing.T) {
	withTestStdin(t, "line one\nline two\n")

	got, err := readPromptSource(rootOptions{})
	if err != nil {
		t.Fatalf("readPromptSource failed: %v", err)
	}
	if got.instructionText != "" || got.stdinText != "line one\nline two" {
		t.Fatalf("unexpected prompt source: %+v", got)
	}
}

func TestReadPromptCombinesPromptAndStdin(t *testing.T) {
	withTestStdin(t, "error line\nstack trace\n")

	got, err := readPromptSource(rootOptions{Prompt: []string{"analyze", "these", "logs"}})
	if err != nil {
		t.Fatalf("readPromptSource failed: %v", err)
	}
	if got.instructionText != "analyze these logs" || got.stdinText != "error line\nstack trace" {
		t.Fatalf("unexpected prompt source: %+v", got)
	}
}

func TestReadPromptEditorAndStdinConflict(t *testing.T) {
	withTestStdin(t, "payload\n")

	_, err := readPromptSource(rootOptions{Editor: true})
	if err == nil || err.Error() != "--editor cannot be combined with piped stdin" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadPromptRejectsEmptyStdin(t *testing.T) {
	withTestStdin(t, "\n \n")

	_, err := readPromptSource(rootOptions{})
	if err == nil || err.Error() != "empty prompt from stdin" {
		t.Fatalf("unexpected error: %v", err)
	}
}
