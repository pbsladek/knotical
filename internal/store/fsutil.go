package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const secureDirMode = 0o700
const secureFileMode = 0o600

func resourcePath(dir, name, ext string) (string, error) {
	if err := validateResourceName(name); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+ext), nil
}

func validateResourceName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("resource name cannot have leading or trailing whitespace")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("resource name cannot contain path separators")
	}
	cleaned := filepath.Clean(name)
	if cleaned != name || cleaned == "." || cleaned == ".." {
		return fmt.Errorf("resource name %q is invalid", name)
	}
	return nil
}

func ensureSecureDir(path string) error {
	return os.MkdirAll(path, secureDirMode)
}

func ensureSecureParent(path string) error {
	return ensureSecureDir(filepath.Dir(path))
}

func openSecureFile(path string) (*os.File, error) {
	if err := ensureSecureParent(path); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, secureFileMode)
}
