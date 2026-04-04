package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Fragment struct {
	Name        string `json:"name"`
	Content     string `json:"content"`
	Description string `json:"description,omitempty"`
}

type FragmentStore struct{ Dir string }

func (s FragmentStore) path(name string) (string, error) { return resourcePath(s.Dir, name, ".md") }

func (s FragmentStore) Save(name, content string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := ensureSecureDir(s.Dir); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), secureFileMode)
}

func (s FragmentStore) Load(name string) (Fragment, error) {
	path, err := s.path(name)
	if err != nil {
		return Fragment{}, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return Fragment{}, err
	}
	return Fragment{Name: name, Content: string(payload), Description: summarize(string(payload))}, nil
}

func (s FragmentStore) Delete(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s FragmentStore) Exists(name string) bool {
	path, err := s.path(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func (s FragmentStore) List() ([]Fragment, error) {
	if _, err := os.Stat(s.Dir); os.IsNotExist(err) {
		return []Fragment{}, nil
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	fragments := []Fragment{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		fragment, err := s.Load(strings.TrimSuffix(entry.Name(), ".md"))
		if err == nil {
			fragments = append(fragments, fragment)
		}
	}
	slices.SortFunc(fragments, func(a, b Fragment) int { return strings.Compare(a.Name, b.Name) })
	return fragments, nil
}

func summarize(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > 80 {
			return string(runes[:80])
		}
		return line
	}
	return ""
}
