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
