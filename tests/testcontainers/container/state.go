package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Entry is a running container tracked by name in the state file.
type Entry struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`              // "postgres", "nats", "redis"
	ID        string    `json:"id"`                // Docker container ID
	DSN       string    `json:"dsn"`               // primary connection string
	Image     string    `json:"image"`             // image used
	Network   string    `json:"network,omitempty"` // Docker network name (if any)
	CreatedAt time.Time `json:"created_at"`
}

type state struct {
	Containers map[string]Entry `json:"containers"`
}

func statePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gx", "containers.json"), nil
}

// withLock acquires an exclusive file lock around fn to prevent concurrent
// read-modify-write races from two gx invocations running in parallel.
func withLock(fn func() error) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck // unlock on close is best-effort
	return fn()
}

func loadState() (state, error) {
	path, err := statePath()
	if err != nil {
		return state{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state{Containers: map[string]Entry{}}, nil
	}
	if err != nil {
		return state{}, err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}, err
	}
	if s.Containers == nil {
		s.Containers = map[string]Entry{}
	}
	return s, nil
}

// saveState writes state atomically: write to a temp file then rename over the
// final path so a mid-write kill never leaves a corrupt file.
func saveState(s state) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".containers-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// GetEntry returns the named container entry, or an error if not found.
func GetEntry(name string) (Entry, error) {
	s, err := loadState()
	if err != nil {
		return Entry{}, err
	}
	e, ok := s.Containers[name]
	if !ok {
		return Entry{}, fmt.Errorf("no container named %q — run: gx container create <type> -n %s", name, name)
	}
	return e, nil
}

// ListEntries returns all tracked containers.
func ListEntries() ([]Entry, error) {
	s, err := loadState()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(s.Containers))
	for _, e := range s.Containers {
		out = append(out, e)
	}
	return out, nil
}

func addEntry(e Entry) error {
	return withLock(func() error {
		s, err := loadState()
		if err != nil {
			return err
		}
		if _, exists := s.Containers[e.Name]; exists {
			return fmt.Errorf("container named %q already exists — remove it first: gx container rm %s", e.Name, e.Name)
		}
		s.Containers[e.Name] = e
		return saveState(s)
	})
}

func removeEntry(name string) error {
	return withLock(func() error {
		s, err := loadState()
		if err != nil {
			return err
		}
		delete(s.Containers, name)
		return saveState(s)
	})
}
