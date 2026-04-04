package store

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type KeyManager struct {
	store JSONMapStore
}

func NewKeyManager(path string) KeyManager {
	return KeyManager{store: JSONMapStore{Path: path}}
}

func (m KeyManager) Set(provider, key string) error {
	values, err := m.store.Load()
	if err != nil {
		return err
	}
	values[strings.ToLower(provider)] = key
	return m.store.Save(values)
}

func (m KeyManager) Get(provider string) (string, bool, error) {
	values, err := m.store.Load()
	if err != nil {
		return "", false, err
	}
	value, ok := values[strings.ToLower(provider)]
	return value, ok, nil
}

func (m KeyManager) Require(provider string) (string, error) {
	if key, err := os.LookupEnv(strings.ToUpper(provider) + "_API_KEY"); err && key != "" {
		return key, nil
	}
	if key := os.Getenv("KNOTICAL_" + strings.ToUpper(provider) + "_API_KEY"); key != "" {
		return key, nil
	}
	value, ok, err := m.Get(provider)
	if err != nil {
		return "", err
	}
	if !ok || value == "" {
		return "", fmt.Errorf("no API key found for provider %q", provider)
	}
	return value, nil
}

func (m KeyManager) Remove(provider string) (bool, error) {
	values, err := m.store.Load()
	if err != nil {
		return false, err
	}
	provider = strings.ToLower(provider)
	if _, ok := values[provider]; !ok {
		return false, nil
	}
	delete(values, provider)
	return true, m.store.Save(values)
}

func (m KeyManager) ListStored() ([]string, error) {
	values, err := m.store.Load()
	if err != nil {
		return nil, err
	}
	providers := make([]string, 0, len(values))
	for provider := range values {
		providers = append(providers, provider)
	}
	slices.Sort(providers)
	return providers, nil
}

func MaskKey(key string) string {
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func PromptHidden(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	value, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	return string(value), err
}
