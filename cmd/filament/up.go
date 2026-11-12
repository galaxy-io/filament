package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
)

func (a *cliApp) upCommand() *cobra.Command {
	var addr string
	var detach bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Run the local engine and web UI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected, err := a.selectContext()
			if err != nil {
				return err
			}
			if selected.Target.Kind == contexts.KindRemote {
				return fmt.Errorf("context %s is remote; up serves local contexts", selected.Name)
			}
			a.configPath = selected.Target.ConfigPath
			return a.up(cmd.Context(), addr, detach)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "Serve the UI and API on `ADDR`")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in the background; stop with filament down")
	return cmd
}

func (a *cliApp) downCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the local engine started by up",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return a.down()
		},
	}
}

// up runs the local engine with the web UI on a stable address until
// interrupted, recording it so other local commands attach instead of
// starting their own. The document is applied once on start; the engine's
// store is the state from then on.
func (a *cliApp) up(ctx context.Context, addr string, detach bool) error {
	if state, running := a.readUpState(); running {
		return fmt.Errorf("already running (pid %d) on http://%s; stop it with filament down", state.PID, state.Addr)
	}
	local, err := a.engine(ctx)
	if err != nil {
		return err
	}
	process, err := local.Start(ctx, addr, detach)
	if err != nil {
		return err
	}
	stop := func() error {
		if detach {
			return syscall.Kill(process.PID, syscall.SIGTERM)
		}
		return process.Stop()
	}
	if _, err := a.applyLocal(ctx, local, process.Endpoint); err != nil {
		return errors.Join(err, stop())
	}
	if err := a.writeUpState(upState{PID: process.PID, Addr: addr, Config: a.configPath}); err != nil {
		return errors.Join(err, stop())
	}
	mode := "Ctrl-C to stop."
	if detach {
		mode = "Stop with filament down."
	}
	if _, err := fmt.Fprintf(a.stdout, "filament up (pid %d)\n  ui:     %s\n  api:    %s\n  config: %s\n  state:  %s\n  log:    %s\n%s\n",
		process.PID, process.Endpoint, process.Endpoint, a.configPath, a.dbPath(), local.LogPath(), mode); err != nil || detach {
		return err
	}
	defer func() { _ = os.Remove(a.upStatePath()) }()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	exited := make(chan error, 1)
	go func() { exited <- process.Wait() }()
	select {
	case err := <-exited:
		return err
	case <-ctx.Done():
		return process.Stop()
	}
}

// upState records the running engine so other commands, down, and status
// can find it. One engine per state directory.
type upState struct {
	PID    int    `json:"pid"`
	Addr   string `json:"addr"`
	Config string `json:"config"`
}

func (a *cliApp) upStatePath() string { return filepath.Join(a.stateDir(), "up.json") }

// down stops the engine recorded by up.
func (a *cliApp) down() error {
	state, running := a.readUpState()
	if !running {
		_ = os.Remove(a.upStatePath())
		return errors.New("no local engine is running; start one with filament up -d")
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
	return os.WriteFile(a.upStatePath(), data, 0o600)
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
