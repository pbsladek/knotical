package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type Role struct {
	Name             string `toml:"name"`
	SystemPrompt     string `toml:"system_prompt"`
	Description      string `toml:"description,omitempty"`
	PrettifyMarkdown bool   `toml:"prettify_markdown"`
}

type RoleStore struct{ Dir string }

func (s RoleStore) path(name string) (string, error) { return resourcePath(s.Dir, name, ".toml") }

func BuiltinRoles() map[string]Role {
	return map[string]Role{
		"default": {
			Name:             "default",
			SystemPrompt:     "You are a helpful assistant. Answer concisely and accurately.",
			Description:      "General-purpose assistant",
			PrettifyMarkdown: true,
		},
		"shell": {
			Name:             "shell",
			SystemPrompt:     "Provide only a single shell command as output with no explanation. Output the command for the current OS and shell. Do NOT use markdown code blocks.",
			Description:      "Generate shell commands only",
			PrettifyMarkdown: false,
		},
		"code": {
			Name:             "code",
			SystemPrompt:     "Provide only code as output without any explanation or markdown formatting. Do not add backticks or language tags around the code.",
			Description:      "Generate raw code only",
			PrettifyMarkdown: false,
		},
	}
}

func (s RoleStore) Save(role Role) error {
	path, err := s.path(role.Name)
	if err != nil {
		return err
	}
	file, err := openSecureFile(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(role)
}

func (s RoleStore) Load(name string) (Role, error) {
	path, err := s.path(name)
	if err != nil {
		return Role{}, err
	}
	if role, ok := BuiltinRoles()[name]; ok {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return role, nil
		}
	}
	var role Role
	_, err = toml.DecodeFile(path, &role)
	return role, err
}

func (s RoleStore) Delete(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s RoleStore) List() ([]Role, error) {
	merged := BuiltinRoles()
	if _, err := os.Stat(s.Dir); err == nil {
		entries, err := os.ReadDir(s.Dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
				continue
			}
			role, err := s.Load(strings.TrimSuffix(entry.Name(), ".toml"))
			if err == nil {
				merged[role.Name] = role
			}
		}
	}
	roles := make([]Role, 0, len(merged))
	for _, role := range merged {
		roles = append(roles, role)
	}
	slices.SortFunc(roles, func(a, b Role) int { return strings.Compare(a.Name, b.Name) })
	return roles, nil
}
