// Package scheduler is the cron registry + intake module — the control plane that
// fires runs on a schedule. It owns schedule lifecycle (register/update/pause/
// resume/delete) over a filament.ScheduleStore, computes the next fire time with the
// dependency-free cron parser, and on each tick claims due schedules and compiles
// their pipelines into runs. It declares no bus subscriptions; its driver is a timer, started
// explicitly by the composition root via Start.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
	"github.com/galaxy-io/filament/module"
)

// defaultInterval is how often the timer claims due schedules.
const defaultInterval = time.Second

// Module is the schedule registry and run-firing timer.
type Module struct {
	store     filament.ScheduleStore
	bus       eventbus.Bus
	ds        filament.DataStore
	log       filament.Logger
	mx        filament.Metrics
	interval  time.Duration
	pipelines PipelineSubmitter
}

// PipelineSubmitter compiles a pipeline schedule occurrence into concrete runs.
type PipelineSubmitter interface {
	SubmitScheduledPipeline(context.Context, string, filament.ScheduleID, string) ([]filament.RunID, error)
}

// Option configures a Module.
type Option func(*Module)

// WithInterval sets the claim cadence of the timer started by Start (default 1s).
func WithInterval(d time.Duration) Option { return func(m *Module) { m.interval = d } }

// WithPipelineSubmitter enables pipeline-linked schedules.
func WithPipelineSubmitter(p PipelineSubmitter) Option { return func(m *Module) { m.pipelines = p } }

// New returns an unmounted scheduler over the given ScheduleStore. Bus/DataStore
// are injected by Mount; the timer is launched by Start.
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
	m.log = d.Log
	m.mx = d.Metrics
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
						m.log.Error("scheduler: tick", err)
					}
				}
			}
		}
	}()
}

// Register validates the spec's cron, computes the first fire time, and persists
// the schedule, returning its id.
func (m *Module) Register(ctx context.Context, spec filament.ScheduleSpec) (filament.ScheduleID, error) {
	next, err := scheduledomain.NextFire(spec, time.Now())
	if err != nil {
		return "", err
	}
	if !spec.Enabled {
		next = nil
	}
	id := newScheduleID()
	st := filament.ScheduleState{
		ID:        id,
		Spec:      spec,
		Enabled:   spec.Enabled,
		NextFire:  next,
		CreatedAt: time.Now(),
	}
	if err := m.store.SaveSchedule(ctx, st); err != nil {
		return "", err
	}
	return id, nil
}

// Update replaces a schedule's spec and recomputes its next fire time.
func (m *Module) Update(ctx context.Context, id filament.ScheduleID, spec filament.ScheduleSpec) error {
	st, err := m.store.LoadSchedule(ctx, id)
	if err != nil {
		return err
	}
	next, err := scheduledomain.NextFire(spec, time.Now())
	if err != nil {
		return err
	}
	if !spec.Enabled {
		next = nil
	}
	st.Spec = spec
	st.Enabled = spec.Enabled
	st.NextFire = next
	return m.store.SaveSchedule(ctx, st)
}

// Get returns one schedule's state.
func (m *Module) Get(ctx context.Context, id filament.ScheduleID) (filament.ScheduleState, error) {
	return m.store.LoadSchedule(ctx, id)
}

// List returns schedules matching the filter.
func (m *Module) List(ctx context.Context, f filament.ScheduleFilter) ([]filament.ScheduleState, error) {
	return m.store.ListSchedules(ctx, f)
}

// Pause disables a schedule so the timer stops claiming it.
func (m *Module) Pause(ctx context.Context, id filament.ScheduleID) error {
	return m.setEnabled(ctx, id, false)
}

// Resume re-enables a schedule, recomputing its next fire from now.
func (m *Module) Resume(ctx context.Context, id filament.ScheduleID) error {
	st, err := m.store.LoadSchedule(ctx, id)
	if err != nil {
		return err
	}
	next, err := scheduledomain.NextFire(st.Spec, time.Now())
	if err != nil {
		return err
	}
	st.Enabled = true
	st.Spec.Enabled = true
	st.NextFire = next
	return m.store.SaveSchedule(ctx, st)
}

// Delete removes a schedule.
func (m *Module) Delete(ctx context.Context, id filament.ScheduleID) error {
	return m.store.DeleteSchedule(ctx, id)
}

func (m *Module) setEnabled(ctx context.Context, id filament.ScheduleID, on bool) error {
	st, err := m.store.LoadSchedule(ctx, id)
	if err != nil {
		return err
	}
	st.Enabled = on
	st.Spec.Enabled = on
	if !on {
		st.NextFire = nil
	}
	return m.store.SaveSchedule(ctx, st)
}

// runDue claims schedules due at now and fires each, returning how many ran. A
// schedule whose previous run is still active is skipped under OverlapSkip but
// still advances its next fire, so it is not re-claimed every tick. Firing a run
// advances the schedule and emits a schedule.fired fact. Returned only on a
// claim/store error; a single run's failure is logged and the loop continues.
func (m *Module) runDue(ctx context.Context, now time.Time) (int, error) {
	due, err := m.store.ClaimDue(ctx, now, 0)
	if err != nil {
		return 0, err
	}
	fired := 0
	for _, st := range due {
		if st.Spec.Overlap == filament.OverlapSkip && m.hasActiveRuns(ctx, st.ID) {
			if m.mx != nil {
				m.mx.Counter("filament_schedule_overlap_skips_total").Inc()
			}
			m.advance(ctx, st, now)
			continue
		}
		if m.pipelines == nil {
			_ = m.store.ReleaseScheduleClaim(ctx, st.ID)
			return fired, fmt.Errorf("scheduler: pipeline submitter is not configured")
		}
		occurrence := now
		if st.NextFire != nil {
			occurrence = *st.NextFire
		}
		ids, err := m.pipelines.SubmitScheduledPipeline(
			ctx,
			st.Spec.PipelineID,
			st.ID,
			fmt.Sprintf("%s:%d", st.ID, occurrence.Unix()),
		)
		var runID filament.RunID
		if len(ids) > 0 {
			runID = ids[0]
		}
		if err != nil {
			if m.mx != nil {
				m.mx.Counter("filament_schedule_submit_failures_total").Inc()
			}
			if m.log != nil {
				m.log.Error("scheduler: submit", err, filament.Field{Key: "schedule", Value: string(st.ID)})
			}
			if releaseErr := m.store.ReleaseScheduleClaim(ctx, st.ID); releaseErr != nil {
				return fired, releaseErr
			}
			continue
		}

		t := now
		st.LastFired = &t
		if next, err := scheduledomain.NextFire(st.Spec, now); err == nil {
			st.NextFire = next
		}
		if err := m.store.SaveSchedule(ctx, st); err != nil {
			return fired, err
		}

		_ = events.Emit(ctx, m.bus, events.ScheduleFired,
			events.Envelope{Tenant: st.Spec.Tenant, Run: runID, At: now}, events.ScheduleFiredEvent{})
		if m.mx != nil {
			m.mx.Counter("filament_schedule_fires_total").Inc()
		}
		fired++
	}
	return fired, nil
}

// advance recomputes and persists a schedule's next fire without firing it.
func (m *Module) advance(ctx context.Context, st filament.ScheduleState, now time.Time) {
	if next, err := scheduledomain.NextFire(st.Spec, now); err == nil {
		st.NextFire = next
		_ = m.store.SaveSchedule(ctx, st)
	}
}

// hasActiveRuns checks every route run produced by prior occurrences.
func (m *Module) hasActiveRuns(ctx context.Context, scheduleID filament.ScheduleID) bool {
	active, err := m.ds.ListRuns(ctx, filament.RunFilter{
		Schedule: scheduleID,
		Status: []filament.RunStatus{
			filament.RunRequested,
			filament.RunRunning,
			filament.RunPaused,
			filament.RunPartial,
		},
		Limit: 1,
	})
	return err == nil && len(active) > 0
}

func newScheduleID() filament.ScheduleID {
	return filament.ScheduleID(uuid.NewString())
}
