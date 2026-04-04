package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type Template struct {
	Name         string   `toml:"name"`
	SystemPrompt string   `toml:"system_prompt,omitempty"`
	Model        string   `toml:"model,omitempty"`
	Temperature  *float64 `toml:"temperature,omitempty"`
	Description  string   `toml:"description,omitempty"`
}

type TemplateStore struct{ Dir string }

func (s TemplateStore) path(name string) (string, error) { return resourcePath(s.Dir, name, ".toml") }

func (s TemplateStore) Path(name string) (string, error) { return s.path(name) }

func (s TemplateStore) Save(template Template) error {
	path, err := s.path(template.Name)
	if err != nil {
		return err
	}
	file, err := openSecureFile(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(template)
}

func (s TemplateStore) Load(name string) (Template, error) {
	var template Template
	path, err := s.path(name)
	if err != nil {
		return Template{}, err
	}
	_, err = toml.DecodeFile(path, &template)
	return template, err
}

func (s TemplateStore) Delete(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s TemplateStore) Exists(name string) bool {
	path, err := s.path(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func (s TemplateStore) List() ([]Template, error) {
	if _, err := os.Stat(s.Dir); os.IsNotExist(err) {
		return []Template{}, nil
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	templates := []Template{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		template, err := s.Load(strings.TrimSuffix(entry.Name(), ".toml"))
		if err == nil {
			templates = append(templates, template)
		}
	}
	slices.SortFunc(templates, func(a, b Template) int { return strings.Compare(a.Name, b.Name) })
	return templates, nil
}
