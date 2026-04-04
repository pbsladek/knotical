package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pbsladek/knotical/internal/model"
)

type CacheEntry struct {
	Response string `json:"response"`
}

type CacheStore struct{ Dir string }

func (s CacheStore) path(model string, system string, messages []model.Message, schema map[string]any, temperature *float64, topP *float64) (string, error) {
	if err := ensureSecureDir(s.Dir); err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"model":       model,
		"system":      system,
		"messages":    messages,
		"schema":      schema,
		"temperature": temperature,
		"top_p":       topP,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return filepath.Join(s.Dir, hex.EncodeToString(sum[:])+".json"), nil
}

func (s CacheStore) Get(model string, system string, messages []model.Message, schema map[string]any, temperature *float64, topP *float64) (string, bool, error) {
	path, err := s.path(model, system, messages, schema, temperature, topP)
	if err != nil {
		return "", false, err
	}
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return "", false, err
	}
	return entry.Response, true, nil
}

func (s CacheStore) Set(model string, system string, messages []model.Message, schema map[string]any, temperature *float64, topP *float64, response string) error {
	path, err := s.path(model, system, messages, schema, temperature, topP)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(CacheEntry{Response: response}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, secureFileMode)
}
