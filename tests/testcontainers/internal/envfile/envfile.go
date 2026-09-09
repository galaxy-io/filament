// Package envfile loads image pins shared by Compose and the container helpers.
package envfile

import (
	"bufio"
	_ "embed"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed images.env
var imageDefaults string

// FindEnv walks to the repository root so nested modules share overrides.
func FindEnv() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yaml")); err == nil {
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
	return parse(f)
}

func parse(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
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

// Images loads embedded pins, optional repository overrides, then environment
// overrides. Compiled tests also work outside the source tree.
func Images() (map[string]string, error) {
	values, err := parse(strings.NewReader(imageDefaults))
	if err != nil {
		return nil, err
	}
	path, err := FindEnv()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		overrides, readErr := Parse(path)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return nil, readErr
		}
		for key, value := range overrides {
			values[key] = value
		}
	}
	for key := range values {
		if value := os.Getenv(key); value != "" {
			values[key] = value
		}
	}
	return values, nil
}
