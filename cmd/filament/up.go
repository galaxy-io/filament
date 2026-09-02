package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
	remotetarget "github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
	"github.com/galaxy-io/filament/ui"
)

func (a *cliApp) upCommand() *cobra.Command {
	var addr string
	var detach bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Run the local deployment and web UI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.resolveLocalConfig(); err != nil {
				return err
			}
			if detach {
				return a.upDetached(addr)
			}
			return a.up(cmd.Context(), addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "Serve the UI and API on `ADDR`")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in the background; stop with filament down")
	return cmd
}

func (a *cliApp) downCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the detached local deployment",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return a.down()
		},
	}
}

// up serves the same embedded deployment every local command boots, but on a
// stable address with the web UI mounted, and keeps it running. State lives
// in memory: the YAML document seeds it and outlives it.
func (a *cliApp) up(ctx context.Context, addr string) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	secrets := newEmbeddedSecrets()
	done := make(chan error, 1)
	go func() {
		done <- app.Serve(ctx, listener, app.WithSecrets(secrets), app.WithUI(ui.Handler()))
	}()
	endpoint := "http://" + listener.Addr().String()
	target := localtarget.NewTarget(
		localtarget.Store{Path: a.configPath},
		remotetarget.NewTarget(remotetarget.Options{Endpoint: endpoint}),
	)
	if err := target.Apply(ctx); err != nil {
		stop()
		<-done
		return fmt.Errorf("apply %s: %w", a.configPath, err)
	}
	syncer, err := localtarget.NewSyncer(target, secrets)
	if err != nil {
		stop()
		<-done
		return fmt.Errorf("sync %s: %w", a.configPath, err)
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := syncer.Tick(ctx); err != nil && ctx.Err() == nil {
					_, _ = fmt.Fprintf(a.statusWriter(), "sync: %v\n", err)
				}
			}
		}
	}()
	if _, err := fmt.Fprintf(a.stdout, "filament up\n  ui:     %s\n  api:    %s\n  config: %s\nCtrl-C to stop.\n", endpoint, endpoint, a.configPath); err != nil {
		stop()
		<-done
		return err
	}
	return <-done
}

// upState records the detached deployment so down can find it. One detached
// deployment per config directory.
type upState struct {
	PID    int    `json:"pid"`
	Addr   string `json:"addr"`
	Config string `json:"config"`
}

func (a *cliApp) upStatePath() string { return filepath.Join(filepath.Dir(a.contextPath), "up.json") }
func (a *cliApp) upLogPath() string   { return filepath.Join(filepath.Dir(a.contextPath), "up.log") }

// upDetached re-executes this binary as a session-leader child running up in
// the foreground, waits for it to answer, and records it for down.
func (a *cliApp) upDetached(addr string) error {
	if state, running := a.readUpState(); running {
		return fmt.Errorf("already running (pid %d) on http://%s; stop it with filament down", state.PID, state.Addr)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.upLogPath()), 0o750); err != nil {
		return err
	}
	logFile, err := os.OpenFile(a.upLogPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	// #nosec G204 -- deliberately re-executes this same binary in the background
	child := exec.Command(executable, "up", "--addr", addr, "--config", a.configPath)
	child.Stdout = logFile
	child.Stderr = logFile
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		return err
	}
	state := upState{PID: child.Process.Pid, Addr: addr, Config: a.configPath}
	if err := a.writeUpState(state); err != nil {
		_ = child.Process.Signal(syscall.SIGTERM)
		return err
	}
	if err := awaitUp("http://"+addr, child.Process.Pid); err != nil {
		_ = os.Remove(a.upStatePath())
		return fmt.Errorf("%w; see %s", err, a.upLogPath())
	}
	_, err = fmt.Fprintf(a.stdout, "filament up (detached, pid %d)\n  ui:     http://%s\n  api:    http://%s\n  config: %s\n  log:    %s\nStop with filament down.\n",
		state.PID, addr, addr, a.configPath, a.upLogPath())
	return err
}

// down stops the detached deployment recorded by up -d.
func (a *cliApp) down() error {
	state, running := a.readUpState()
	if !running {
		_ = os.Remove(a.upStatePath())
		return errors.New("no detached deployment is running; start one with filament up -d")
	}
	if err := syscall.Kill(state.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop pid %d: %w", state.PID, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(state.PID) {
			_ = os.Remove(a.upStatePath())
			_, err := fmt.Fprintf(a.stdout, "filament down (pid %d stopped)\n", state.PID)
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("pid %d did not exit within 10s; kill it manually", state.PID)
}

func (a *cliApp) readUpState() (upState, bool) {
	data, err := os.ReadFile(a.upStatePath())
	if err != nil {
		return upState{}, false
	}
	var state upState
	if err := json.Unmarshal(data, &state); err != nil || state.PID <= 0 {
		return upState{}, false
	}
	return state, processAlive(state.PID)
}

func (a *cliApp) writeUpState(state upState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.upStatePath()), 0o750); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return os.WriteFile(a.upStatePath(), data, 0o600)
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// awaitUp polls the deployment until it answers, the child dies, or 15s pass.
func awaitUp(endpoint string, pid int) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return errors.New("deployment exited during startup")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
		if err == nil {
			response, err := http.DefaultClient.Do(request)
			if err == nil {
				_ = response.Body.Close()
				cancel()
				return nil
			}
		}
		cancel()
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("deployment did not answer within 15s")
}

// resolveLocalConfig points up at the selected context's document. up only
// serves local contexts; a remote selection is an error rather than a silent
// fallback.
func (a *cliApp) resolveLocalConfig() error {
	selected, err := a.contextRegistry().Resolve(a.contextName)
	if err != nil {
		return err
	}
	if selected.Target.Kind == contexts.KindRemote {
		return fmt.Errorf("context %s is remote; up serves local contexts", selected.Name)
	}
	if !a.configOverride && selected.Target.ConfigPath != "" {
		a.configPath = selected.Target.ConfigPath
	}
	return nil
}
