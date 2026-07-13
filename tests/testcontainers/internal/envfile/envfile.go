// Package envfile locates and parses docker/.env files shared between
// testcontainers and the gx CLI so both always reference the same image tags.
package envfile

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FindEnv walks up from the working directory to the nearest go.mod and
// returns the path to docker/.env beneath it.
func FindEnv() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "docker", ".env"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// Parse reads KEY=VALUE lines from path, ignoring blanks and # comments.
func Parse(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return out, sc.Err()
}
