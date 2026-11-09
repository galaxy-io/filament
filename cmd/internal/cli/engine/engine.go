// Package engine runs filament-standalone as the CLI's local deployment. The
// CLI never executes runs itself: saved pipelines run in a standalone process
// serving the state directory, and inline runs in a short-lived `standalone
// run` beside it.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/mod/semver"

	"github.com/galaxy-io/filament/cmd/internal/cli/upgrade"
)

// BinaryName is the release name of the standalone deployment.
const BinaryName = "filament-standalone"

// Engine is the standalone binary bound to one state directory.
type Engine struct {
	Binary   string
	StateDir string
}

// Locate finds the standalone binary for this CLI version: FILAMENT_ENGINE
// when set, then one beside the running CLI, then the version's cached copy.
// A release build downloads a missing copy, reporting progress to status.
func Locate(ctx context.Context, version string, status io.Writer) (string, error) {
	if path := os.Getenv("FILAMENT_ENGINE"); path != "" {
		return path, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate filament executable: %w", err)
	}
	sibling := filepath.Join(filepath.Dir(executable), BinaryName)
	if _, err := os.Stat(sibling); err == nil {
		return sibling, nil
	}
	cached, err := CachePath(version)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	if !semver.IsValid("v" + strings.TrimPrefix(version, "v")) {
		return "", fmt.Errorf("%s not found beside %s; build it with `go build -o %s ./cmd/standalone` or set FILAMENT_ENGINE", BinaryName, executable, sibling)
	}
	if err := Install(ctx, version, cached, status); err != nil {
		return "", err
	}
	return cached, nil
}

// CachePath is where version's downloaded engine lives.
func CachePath(version string) (string, error) {
	root := os.Getenv("XDG_DATA_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		root = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(root, "filament", "engine", strings.TrimPrefix(version, "v"), BinaryName), nil
}

// Install downloads version's engine to path and verifies it.
func Install(ctx context.Context, version, path string, status io.Writer) error {
	if _, err := fmt.Fprintf(status, "Downloading %s %s...\n", BinaryName, version); err != nil {
		return err
	}
	return upgrade.Install(ctx, upgrade.InstallOptions{
		Client: &http.Client{Timeout: 5 * time.Minute}, Version: version, Binary: BinaryName, Destination: path,
	})
}

// LogPath is where a background engine writes its output.
func (e Engine) LogPath() string { return filepath.Join(e.StateDir, "engine.log") }

// Process is a running engine.
type Process struct {
	Endpoint string
	PID      int
	cmd      *exec.Cmd
	done     chan error
}

// Start serves the state directory on addr and waits until it answers. A
// detached engine outlives this process; stop it by PID.
func (e Engine) Start(ctx context.Context, addr string, detach bool) (*Process, error) {
	if addr == "" {
		var err error
		if addr, err = freeAddr(); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(e.StateDir, 0o700); err != nil {
		return nil, err
	}
	log, err := os.OpenFile(e.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() { _ = log.Close() }()
	// #nosec G204 -- runs the resolved standalone binary with fixed arguments
	cmd := exec.Command(e.Binary)
	cmd.Env = append(os.Environ(),
		"STORE_PATH="+filepath.Join(e.StateDir, "filament.db"),
		"NATS_STORE_DIR="+filepath.Join(e.StateDir, "nats"),
		"INGESTION_ADDR="+addr,
	)
	cmd.Stdout, cmd.Stderr = log, log
	// A detached engine gets its own session so it outlives the terminal. A
	// per-command engine shares the CLI's, so closing the terminal stops both
	// rather than leaving an engine holding the store.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: detach}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", e.Binary, err)
	}
	process := &Process{Endpoint: "http://" + addr, PID: cmd.Process.Pid, cmd: cmd, done: make(chan error, 1)}
	go func() { process.done <- cmd.Wait() }()
	if err := process.await(ctx); err != nil {
		_ = process.Stop()
		return nil, fmt.Errorf("%w; see %s", err, e.LogPath())
	}
	if detach {
		_ = cmd.Process.Release()
	}
	return process, nil
}

// await polls the engine until it answers, exits, or 30s pass.
func (p *Process) await(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-p.done:
			p.done <- err
			return fmt.Errorf("engine exited during startup: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if Answers(p.Endpoint) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("engine did not answer within 30s")
}

// Wait blocks until the engine exits.
func (p *Process) Wait() error {
	err := <-p.done
	p.done <- err
	return err
}

// Stop asks the engine to shut down and waits up to 10s before killing it.
func (p *Process) Stop() error {
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	select {
	case <-p.done: //nolint:errcheck // a signalled exit is the expected outcome
		return nil
	case <-time.After(10 * time.Second):
		return p.cmd.Process.Kill()
	}
}

// Answers reports whether the endpoint returns any HTTP response.
func Answers(endpoint string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false
	}
	_ = response.Body.Close()
	return true
}

func freeAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("find a free port: %w", err)
	}
	addr := listener.Addr().String()
	return addr, listener.Close()
}
