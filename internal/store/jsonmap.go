package store

import (
	"encoding/json"
	"os"
)

type JSONMapStore struct {
	Path string
}

func (s JSONMapStore) Load() (map[string]string, error) {
	if _, err := os.Stat(s.Path); os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	payload, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	if len(payload) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s JSONMapStore) Save(values map[string]string) error {
	if err := ensureSecureParent(s.Path); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, payload, secureFileMode)
}
