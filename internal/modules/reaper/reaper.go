// Package reaper is the zombie-run janitor: on each tick it finds runs stuck
// in Requested or Running with no write in the staleness window — a worker
// that never got admitted, a lost pod, an OOM-killed worker, a crashed engine
// — and emits run.failed so the tracker folds them terminal. Disaster recovery
// only; it plays no part in the normal lifecycle. Partial runs are left alone:
// they hold resumable checkpoints.
package reaper

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
)

const (
	defaultInterval = time.Minute
	// defaultStaleAfter is ten missed heartbeats at the default cadence.
	defaultStaleAfter = 5 * time.Minute
)

// WorkloadProbe reports what the dispatcher can prove about a run's worker.
type WorkloadProbe func(ctx context.Context, run filament.RunID) (filament.Workload, error)

// Module is the staleness sweep over the run store.
type Module struct {
	ds    filament.DataStore
	bus   eventbus.Bus
	log   filament.Logger
	mx    filament.Metrics
	probe WorkloadProbe

	interval   time.Duration
	staleAfter time.Duration
}

// Option configures a Module.
type Option func(*Module)

// WithInterval sets the sweep cadence (default 1m).
func WithInterval(d time.Duration) Option { return func(m *Module) { m.interval = d } }

// WithStaleAfter sets how long a Running run may go without a write before it
// is declared dead (default 5m).
func WithStaleAfter(d time.Duration) Option { return func(m *Module) { m.staleAfter = d } }

// WithWorkloadProbe sets the dispatcher probe consulted before each kill. A
// stale run whose workload is still up is held, since the worker may be alive
// with its heartbeats lost, and a Requested run is only judged dead once the
// probe proves its workload ran and finished. Without one, staleness alone
// decides and only Running runs are swept.
func WithWorkloadProbe(f WorkloadProbe) Option { return func(m *Module) { m.probe = f } }

// New returns an unmounted reaper; providers are injected by Mount and the
// timer is launched by Start.
func New(opts ...Option) *Module {
	m := &Module{interval: defaultInterval, staleAfter: defaultStaleAfter}
	for _, o := range opts {
		o(m)
	}
	return m
}

// NewFromEnv returns an unmounted reaper configured from REAPER_INTERVAL_SECONDS
// and REAPER_STALE_AFTER_SECONDS; unset or invalid values keep the defaults.
// Explicit options win over the environment.
func NewFromEnv(opts ...Option) *Module {
	var envOpts []Option
	if d := envSeconds("REAPER_INTERVAL_SECONDS"); d > 0 {
		envOpts = append(envOpts, WithInterval(d))
	}
	if d := envSeconds("REAPER_STALE_AFTER_SECONDS"); d > 0 {
		envOpts = append(envOpts, WithStaleAfter(d))
	}
	return New(append(envOpts, opts...)...)
}

func envSeconds(key string) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 0
}

var _ module.Module = (*Module)(nil)

// Name identifies this module.
func (m *Module) Name() string { return "reaper" }

// Subscriptions returns none; the reaper is timer-driven, not fact-driven.
func (m *Module) Subscriptions() []host.Subscription { return nil }

// Mount captures the providers this module uses.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.ds = d.DataStore
	m.bus = d.Bus
	m.log = d.Log
	m.mx = d.Metrics
	return nil
}

// Start launches the sweep timer; it runs until ctx is cancelled.
func (m *Module) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := m.reap(ctx, now); err != nil && m.log != nil {
					m.log.Error("reaper: sweep", err)
				}
			}
		}
	}()
}

// reap fails every stale run the workload evidence says is dead. The kill
// travels the bus as an ordinary run.failed fact, so the tracker remains the
// only writer of terminal state. One run's emit failure doesn't stop the
// sweep; the next tick retries anything still stale.
func (m *Module) reap(ctx context.Context, now time.Time) error {
	statuses := []filament.RunStatus{filament.RunRunning}
	if m.probe != nil {
		// A Requested run can only be judged with dispatch evidence.
		statuses = append(statuses, filament.RunRequested)
	}
	stale, _, err := m.ds.ListRuns(ctx, filament.RunFilter{
		Status:        statuses,
		UpdatedBefore: now.Add(-m.staleAfter),
	})
	if err != nil {
		return err
	}
	for _, r := range stale {
		workload := filament.WorkloadAbsent
		if m.probe != nil {
			workload, err = m.probe(ctx, r.Run)
			if err != nil {
				// Can't prove the workload is gone — hold the kill and let the
				// next tick retry rather than fail a run that may still be live.
				if m.log != nil {
					m.log.Error("reaper: workload probe", err, filament.Field{Key: "run", Value: string(r.Run)})
				}
				continue
			}
		}
		if !dead(r.Status, workload) {
			if m.log != nil {
				m.log.Info("reaper: stale run held",
					filament.Field{Key: "run", Value: string(r.Run)},
					filament.Field{Key: "status", Value: int(r.Status)},
					filament.Field{Key: "workload", Value: workload.String()},
					filament.Field{Key: "stale_since", Value: r.UpdatedAt.UTC().Format(time.RFC3339)})
			}
			continue
		}
		err := events.Emit(ctx, m.bus, events.RunFailed,
			events.Envelope{Tenant: r.Tenant, Run: r.Run, At: now},
			events.RunFailedEvent{Error: fmt.Sprintf("reaped: no progress since %s", r.UpdatedAt.UTC().Format(time.RFC3339))})
		if err != nil {
			if m.log != nil {
				m.log.Error("reaper: emit run.failed", err, filament.Field{Key: "run", Value: string(r.Run)})
			}
			continue
		}
		if m.log != nil {
			m.log.Info("reaper: failed stale run",
				filament.Field{Key: "run", Value: string(r.Run)},
				filament.Field{Key: "pipeline", Value: r.Request.PipelineID},
				filament.Field{Key: "stale_since", Value: r.UpdatedAt.UTC().Format(time.RFC3339)})
		}
		if m.mx != nil {
			m.mx.Counter("filament_runs_reaped_total").Inc()
		}
	}
	return nil
}

// dead decides whether a stale run's workload evidence warrants the kill.
// Running dies unless its workload is provably still up; Absent there means
// the platform already collected it. Requested dies only once the workload
// provably ran and finished, because Absent may simply mean not dispatched
// yet.
func dead(status filament.RunStatus, workload filament.Workload) bool {
	switch status {
	case filament.RunRunning:
		return workload != filament.WorkloadActive
	case filament.RunRequested:
		return workload == filament.WorkloadFinished
	default:
		return false
	}
}
