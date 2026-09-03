// Package settings persists CLI preferences that belong to the user rather
// than to a context or a deployment.
package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Settings is the persisted preference file.
type Settings struct {
	// Layout draws tables and menus boxed or plain.
	Layout string `yaml:"layout,omitempty"`
}

// Load reads the settings at path. A missing file is empty settings.
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}
	var out Settings
	if err := yaml.Unmarshal(data, &out); err != nil {
		return Settings{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

// Save writes the settings at path.
func Save(path string, s Settings) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
