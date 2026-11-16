package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// InlineRun is one untracked run executing in `standalone run`.
type InlineRun struct {
	facts   chan events.Fact
	control io.WriteCloser
	done    chan error
}

// Inline starts spec in a short-lived engine process. Sink output goes to
// stdout and stderr, and the run's facts arrive on Facts.
func (e Engine) Inline(ctx context.Context, spec filament.RunSpec, stdout, stderr io.Writer) (*InlineRun, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("encode run spec: %w", err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	// #nosec G204 -- runs the resolved standalone binary with fixed arguments
	cmd := exec.CommandContext(ctx, e.Binary, "run")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.ExtraFiles = []*os.File{writer}
	control, err := cmd.StdinPipe()
	if err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("start %s run: %w", e.Binary, err)
	}
	_ = writer.Close()
	if _, err := control.Write(append(data, '\n')); err != nil {
		_ = cmd.Process.Kill()
		_ = reader.Close()
		return nil, fmt.Errorf("send run spec: %w", err)
	}
	run := &InlineRun{facts: make(chan events.Fact, 64), control: control, done: make(chan error, 1)}
	go func() {
		defer close(run.facts)
		defer func() { _ = reader.Close() }()
		lines := bufio.NewScanner(reader)
		lines.Buffer(make([]byte, 64<<10), 16<<20)
		for lines.Scan() {
			if fact, err := events.Unmarshal(lines.Bytes()); err == nil {
				run.facts <- fact
			}
		}
	}()
	go func() { run.done <- cmd.Wait() }()
	return run, nil
}

// Facts delivers the run's facts and closes when the process exits.
func (r *InlineRun) Facts() <-chan events.Fact { return r.facts }

// Done delivers the process's exit error once.
func (r *InlineRun) Done() <-chan error { return r.done }

// Request asks the run to "pause" or "cancel".
func (r *InlineRun) Request(action string) error {
	_, err := io.WriteString(r.control, action+"\n")
	return err
}

// Close stops sending requests, letting the process exit once the run ends.
func (r *InlineRun) Close() error { return r.control.Close() }
