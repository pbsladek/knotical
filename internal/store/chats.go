package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pbsladek/knotical/internal/model"
)

type ChatStore struct {
	Dir string
}

func (s ChatStore) LoadOrCreate(name string) (model.ChatSession, error) {
	path, err := resourcePath(s.Dir, name, ".json")
	if err != nil {
		return model.ChatSession{}, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return model.NewChatSession(name), nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return model.ChatSession{}, err
	}
	var session model.ChatSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return model.ChatSession{}, err
	}
	return session, nil
}

func (s ChatStore) Save(session model.ChatSession) error {
	path, err := resourcePath(s.Dir, session.Name, ".json")
	if err != nil {
		return err
	}
	if err := ensureSecureDir(s.Dir); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, secureFileMode)
}

func (s ChatStore) Delete(name string) (bool, error) {
	path, err := resourcePath(s.Dir, name, ".json")
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	return true, os.Remove(path)
}

func (s ChatStore) List() ([]string, error) {
	if err := ensureSecureDir(s.Dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())); stem != "" {
			names = append(names, stem)
		}
	}
	slices.Sort(names)
	return names, nil
}
