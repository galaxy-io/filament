package main

import (
	"fmt"
	"os"
	"path/filepath"

	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
	"gopkg.in/yaml.v3"
)

// configStore temporarily preserves the command package's existing API while
// local persistence moves behind the target adapter.
type configStore struct{ path string }

func defaultConfigPath() (string, error) {
	if root := os.Getenv("XDG_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "filament", "filament.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "filament", "filament.yaml"), nil
}

func (s configStore) load() (configDocument, *yaml.Node, error) {
	return s.local().Load()
}

func (s configStore) put(section, name string, value any) error {
	return s.local().Put(section, name, value)
}

func (s configStore) delete(section, name string) error {
	return s.local().Delete(section, name)
}

func (s configStore) write(root *yaml.Node) error {
	return s.local().Write(root)
}

func (s configStore) local() localtarget.Store {
	return localtarget.Store{Path: s.path}
}
