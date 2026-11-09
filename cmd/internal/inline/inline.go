// Package inline runs one untracked transfer in this process over an
// in-memory store and bus, so it leaves no run history and no checkpoints.
// Standalone serves it to the CLI for inline runs and per-run overrides.
//
// The run's facts are written as one JSON line each. Control lines read while
// the run is active request a "pause" or a "cancel".
package inline

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/runner"
)

// Run reads a JSON run spec from the first line of control, executes it, and
// writes every fact of the run to facts until it finishes.
func Run(ctx context.Context, sources filament.SourceRegistry, sinks filament.SinkRegistry, control io.Reader, facts io.Writer) error {
	lines := bufio.NewScanner(control)
	lines.Buffer(make([]byte, 64<<10), 16<<20)
	if !lines.Scan() {
		return fmt.Errorf("inline: read run spec: %w", cmpErr(lines.Err(), io.ErrUnexpectedEOF))
	}
	var spec filament.RunSpec
	if err := json.Unmarshal(lines.Bytes(), &spec); err != nil {
		return fmt.Errorf("inline: decode run spec: %w", err)
	}

	bus := inproc.New(inproc.WithBuffer(1024))
	defer func() { _ = bus.Close() }()
	subscription, err := bus.Subscribe(events.RunPattern(spec.Tenant, spec.Run), eventbus.SubOpts{})
	if err != nil {
		return err
	}
	defer func() { _ = subscription.Close() }()
	store := sqlite.NewMemory()
	defer func() { _ = store.Close() }()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runner.RunOne(runCtx, runner.Deps{Bus: bus, DataStore: store, Sources: sources, Sinks: sinks}, spec)
	}()
	go signals(runCtx, bus, spec, lines)

	for {
		select {
		case message := <-subscription.C():
			if err := forward(message, facts); err != nil {
				return err
			}
		case runErr := <-done:
			// The terminal fact is queued before RunOne returns.
			for {
				select {
				case message := <-subscription.C():
					if err := forward(message, facts); err != nil {
						return err
					}
				default:
					return runErr
				}
			}
		}
	}
}

func forward(message eventbus.Message, facts io.Writer) error {
	fact, err := events.Decode(message)
	_ = message.Ack()
	if err != nil {
		return err
	}
	data, err := events.Marshal(fact)
	if err != nil {
		return err
	}
	_, err = facts.Write(append(data, '\n'))
	return err
}

// signals turns control lines into run lifecycle requests.
func signals(ctx context.Context, bus eventbus.Bus, spec filament.RunSpec, lines *bufio.Scanner) {
	for lines.Scan() {
		envelope := events.Envelope{Tenant: spec.Tenant, Run: spec.Run, At: time.Now()}
		switch lines.Text() {
		case "pause":
			_ = events.Emit(ctx, bus, events.RunPauseRequested, envelope, events.RunPauseRequestedEvent{})
		case "cancel":
			_ = events.Emit(ctx, bus, events.RunCancelRequested, envelope, events.RunCancelRequestedEvent{})
		}
	}
}

func cmpErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
