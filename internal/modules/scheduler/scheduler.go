// Package scheduler is the cron firing module: on each tick it claims due
// schedules, compiles their pipelines into runs, and submits them. Schedule
// lifecycle (create/update/pause/resume/delete) is owned by the API over a
// filament.ScheduleStore; this module only reads schedules, fires them, and
// maintains their pre-created RunScheduled rows. It declares no bus
// subscriptions; its driver is a timer, started explicitly by the composition
// root via Start.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/compile"
	"github.com/galaxy-io/filament/internal/runs"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
	"github.com/galaxy-io/filament/module"
)

// defaultInterval is how often the timer claims due schedules.
const defaultInterval = time.Second

// Module is the run-firing timer over a ScheduleStore.
type Module struct {
	store    filament.ScheduleStore
	bus      eventbus.Bus
	ds       filament.DataStore
	log      filament.Logger
	mx       filament.Metrics
	interval time.Duration
	compiler *compile.Compiler
}

// Option configures a Module.
type Option func(*Module)

// WithInterval sets the claim cadence of the timer started by Start (default 1s).
func WithInterval(d time.Duration) Option { return func(m *Module) { m.interval = d } }

// New returns an unmounted scheduler over the given ScheduleStore. The bus,
// data store, and compiler inputs are injected by Mount; the timer is launched
// by Start.
func New(store filament.ScheduleStore, opts ...Option) *Module {
	m := &Module{store: store, interval: defaultInterval}
	for _, o := range opts {
		o(m)
	}
	return m
}

var _ module.Module = (*Module)(nil)

// Name identifies this module.
func (m *Module) Name() string { return "scheduler" }

// Subscriptions returns none; the scheduler is timer-driven, not fact-driven.
func (m *Module) Subscriptions() []host.Subscription { return nil }

// Mount captures the providers this module uses. Cheap, no I/O.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.bus = d.Bus
	m.ds = d.DataStore
	if d.Log != nil {
		m.log = d.Log.With(filament.Field{Key: "component", Value: "scheduler"})
	}
	m.mx = d.Metrics
	m.compiler = &compile.Compiler{Store: d.DataStore, Sources: d.Sources, Sinks: d.Sinks}
	return nil
}

// Start launches the claim timer; it runs until ctx is cancelled.
func (m *Module) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if _, err := m.runDue(ctx, now); err != nil {
					if m.mx != nil {
						m.mx.Counter("filament_schedule_tick_failures_total").Inc()
					}
					if m.log != nil {
						m.log.Error("scheduler tick failed", err,
							filament.Field{Key: "event.name", Value: "scheduler.tick.failed"})
					}
				}
			}
		}
	}()
}

// runDue claims schedules due at now and fires each, returning how many ran.
// A schedule whose previous run is still active is skipped under OverlapSkip
// but still advances its next fire, so it is not re-claimed every tick. Firing
// advances the schedule and emits a schedule.fired fact. One schedule's
// failure releases its claim and moves on to the rest of the batch; the
// failures come back joined so the tick can count and log them.
func (m *Module) runDue(ctx context.Context, now time.Time) (int, error) {
	due, err := m.store.ClaimDue(ctx, now, 0)
	if err != nil {
		return 0, err
	}
	if len(due) == 0 {
		m.logEmptyTick()
		return 0, nil
	}
	fired := 0
	skipped := 0
	failed := 0
	var errs []error
	for _, st := range due {
		if st.Spec.Overlap == filament.OverlapSkip {
			active, err := m.hasActiveRuns(ctx, st.Spec.Tenant, st.ID)
			if err != nil {
				failed++
				// Can't prove the last occurrence finished — hold the fire and
				// release the claim so the next tick retries, rather than risk
				// an overlapping run.
				errs = append(errs, fmt.Errorf("overlap check %q: %w", st.ID, err))
				if err := m.store.ReleaseScheduleClaim(ctx, st.Spec.Tenant, st.ID); err != nil {
					errs = append(errs, fmt.Errorf("release claim %q: %w", st.ID, err))
				}
				continue
			}
			if active {
				if m.mx != nil {
					m.mx.Counter("filament_schedule_overlap_skips_total").Inc()
				}
				occurrence := ""
				if st.NextFire != nil {
					occurrence = st.NextFire.UTC().Format(time.RFC3339Nano)
				}
				m.reconcileScheduledRuns(ctx, m.advance(ctx, st, now))
				skipped++
				if m.log != nil {
					m.log.Info("scheduled occurrence skipped",
						filament.Field{Key: "event.name", Value: "schedule.occurrence.skipped"},
						filament.Field{Key: "schedule_id", Value: string(st.ID)},
						filament.Field{Key: "pipeline_id", Value: st.Spec.PipelineID},
						filament.Field{Key: "occurrence", Value: occurrence},
						filament.Field{Key: "reason", Value: "overlap"})
				}
				continue
			}
		}
		occurrence := now
		if st.NextFire != nil {
			occurrence = *st.NextFire
		}
		runID, err := m.fire(ctx, st, occurrence)
		if err != nil {
			failed++
			if m.mx != nil {
				m.mx.Counter("filament_schedule_submit_failures_total").Inc()
			}
			errs = append(errs, fmt.Errorf("submit schedule %q: %w", st.ID, err))
			if err := m.store.ReleaseScheduleClaim(ctx, st.Spec.Tenant, st.ID); err != nil {
				errs = append(errs, fmt.Errorf("release claim %q: %w", st.ID, err))
			}
			continue
		}

		t := now
		st.LastFired = &t
		if next, err := scheduledomain.NextFire(st.Spec, now); err == nil {
			st.NextFire = next
		}
		if err := m.store.SaveSchedule(ctx, st); err != nil {
			failed++
			// The fire happened; a stale NextFire is only re-claimed after the
			// lease expires, and the occurrence token dedupes a double fire.
			errs = append(errs, fmt.Errorf("advance %q: %w", st.ID, err))
			continue
		}
		// Reaps any pre-created rows the fire's recompile no longer produced
		// (routes edited away) and pre-creates the next occurrence's runs.
		m.reconcileScheduledRuns(ctx, st)

		if err := events.Emit(ctx, m.bus, events.ScheduleFired,
			events.Envelope{Tenant: st.Spec.Tenant, Run: runID, At: now}, events.ScheduleFiredEvent{}); err != nil && m.log != nil {
			m.log.Error("schedule fired event publish failed", err,
				filament.Field{Key: "event.name", Value: "schedule.fired.publish_failed"},
				filament.Field{Key: "schedule_id", Value: string(st.ID)},
				filament.Field{Key: "run_id", Value: string(runID)})
		}
		if m.mx != nil {
			m.mx.Counter("filament_schedule_fires_total").Inc()
		}
		fired++
		if m.log != nil {
			m.log.Info("scheduled occurrence fired",
				filament.Field{Key: "event.name", Value: "schedule.occurrence.fired"},
				filament.Field{Key: "schedule_id", Value: string(st.ID)},
				filament.Field{Key: "pipeline_id", Value: st.Spec.PipelineID},
				filament.Field{Key: "run_id", Value: string(runID)},
				filament.Field{Key: "occurrence", Value: occurrence})
		}
	}
	m.logTickCompleted(len(due), fired, skipped, failed)
	return fired, errors.Join(errs...)
}

func (m *Module) logEmptyTick() {
	if m.log == nil {
		return
	}
	m.log.Trace("scheduler tick completed",
		filament.Field{Key: "event.name", Value: "scheduler.tick.completed"},
		filament.Field{Key: "claimed_schedules", Value: 0})
}

func (m *Module) logTickCompleted(claimed, fired, skipped, failed int) {
	if m.log == nil {
		return
	}
	m.log.Debug("scheduler tick completed",
		filament.Field{Key: "event.name", Value: "scheduler.tick.completed"},
		filament.Field{Key: "claimed_schedules", Value: claimed},
		filament.Field{Key: "fired_schedules", Value: fired},
		filament.Field{Key: "skipped_schedules", Value: skipped},
		filament.Field{Key: "failed_schedules", Value: failed})
}

// fire compiles the schedule's pipeline for one claimed occurrence and submits
// one run per route, returning the first run id for the schedule.fired fact.
// The occurrence token makes each cron tick idempotent.
func (m *Module) fire(ctx context.Context, st filament.ScheduleState, occurrence time.Time) (filament.RunID, error) {
	compiled, err := m.compiler.Compile(ctx, st.Spec.Tenant, st.Spec.PipelineID, scheduledomain.OccurrenceToken(st.ID, occurrence), filament.RunOptions{}, st.ID, filament.WorkerConfiguration{})
	if err != nil {
		return "", err
	}
	var first filament.RunID
	for _, c := range compiled {
		id, err := runs.Submit(ctx, m.bus, m.ds, c.Submission)
		if err != nil {
			return first, err
		}
		if first == "" {
			first = id
		}
	}
	return first, nil
}

// advance recomputes and persists a schedule's next fire without firing it,
// returning the updated state.
func (m *Module) advance(ctx context.Context, st filament.ScheduleState, now time.Time) filament.ScheduleState {
	if next, err := scheduledomain.NextFire(st.Spec, now); err == nil {
		st.NextFire = next
		if err := m.store.SaveSchedule(ctx, st); err != nil && m.log != nil {
			m.log.Error("schedule advance failed", err,
				filament.Field{Key: "event.name", Value: "schedule.advance.failed"},
				filament.Field{Key: "schedule_id", Value: string(st.ID)})
		}
	}
	return st
}

// reconcileScheduledRuns refreshes the schedule's RunScheduled bookkeeping.
// Failures are logged and counted, not returned: the rows are a visibility
// artifact and must never fail a fire; the counter is what surfaces a
// persistently failing reconcile.
func (m *Module) reconcileScheduledRuns(ctx context.Context, st filament.ScheduleState) {
	if err := runs.ReconcileScheduled(ctx, m.ds, m.compiler, st); err != nil {
		if m.mx != nil {
			m.mx.Counter("filament_schedule_reconcile_failures_total").Inc()
		}
		if m.log != nil {
			m.log.Error("scheduled run reconciliation failed", err,
				filament.Field{Key: "event.name", Value: "schedule.reconcile.failed"},
				filament.Field{Key: "schedule_id", Value: string(st.ID)})
		}
	}
}

// hasActiveRuns checks every route run produced by prior occurrences.
func (m *Module) hasActiveRuns(ctx context.Context, tenant filament.TenantID, scheduleID filament.ScheduleID) (bool, error) {
	active, _, err := m.ds.ListRuns(ctx, filament.RunFilter{
		Tenant:   tenant,
		Schedule: scheduleID,
		Status: []filament.RunStatus{
			filament.RunRequested,
			filament.RunRunning,
			filament.RunPaused,
		},
		Limit: 1,
	})
	if err != nil {
		return false, err
	}
	return len(active) > 0, nil
}
