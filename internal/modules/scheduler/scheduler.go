// Package scheduler is the cron registry + intake module — the control plane that
// fires runs on a schedule. It owns schedule lifecycle (register/update/pause/
// resume/delete) over a ingestion.ScheduleStore, computes the next fire time with the
// dependency-free cron parser, and on each tick claims due schedules and submits
// their runs via the shared runs intake. Immediate one-off runs go through
// Trigger. It declares no bus subscriptions; its driver is a timer, started
// explicitly by the composition root via Start.
package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/cron"
	"github.com/galaxy-io/filament/internal/runs"
	"github.com/galaxy-io/filament/module"
)

// defaultInterval is how often the timer claims due schedules.
const defaultInterval = time.Second

// Module is the schedule registry and run-firing timer.
type Module struct {
	store    ingestion.ScheduleStore
	bus      eventbus.Bus
	ds       ingestion.DataStore
	log      ingestion.Logger
	interval time.Duration
}

// Option configures a Module.
type Option func(*Module)

// WithInterval sets the claim cadence of the timer started by Start (default 1s).
func WithInterval(d time.Duration) Option { return func(m *Module) { m.interval = d } }

// New returns an unmounted scheduler over the given ScheduleStore. Bus/DataStore
// are injected by Mount; the timer is launched by Start.
func New(store ingestion.ScheduleStore, opts ...Option) *Module {
	m := &Module{store: store, interval: defaultInterval}
	for _, o := range opts {
		o(m)
	}
	return m
}

var _ module.Module = (*Module)(nil)

func (m *Module) Name() string { return "scheduler" }

// Subscriptions: none. The scheduler is timer-driven, not fact-driven.
func (m *Module) Subscriptions() []host.Subscription { return nil }

func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.bus = d.Bus
	m.ds = d.DataStore
	m.log = d.Log
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
				if _, err := m.runDue(ctx, now); err != nil && m.log != nil {
					m.log.Error("scheduler: tick", err)
				}
			}
		}
	}()
}

// Trigger submits an immediate one-off run, bypassing scheduling.
func (m *Module) Trigger(ctx context.Context, req ingestion.RunRequest) (ingestion.RunID, error) {
	return runs.Submit(ctx, m.bus, m.ds, req)
}

// Register validates the spec's cron, computes the first fire time, and persists
// the schedule, returning its id.
func (m *Module) Register(ctx context.Context, spec ingestion.ScheduleSpec) (ingestion.ScheduleID, error) {
	if err := spec.Tenant.Valid(); err != nil {
		return "", fmt.Errorf("scheduler: tenant %w", err)
	}
	next, err := nextFire(spec, time.Now())
	if err != nil {
		return "", err
	}
	id := newScheduleID()
	st := ingestion.ScheduleState{
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
func (m *Module) Update(ctx context.Context, id ingestion.ScheduleID, spec ingestion.ScheduleSpec) error {
	st, err := m.store.LoadSchedule(ctx, id)
	if err != nil {
		return err
	}
	next, err := nextFire(spec, time.Now())
	if err != nil {
		return err
	}
	st.Spec = spec
	st.Enabled = spec.Enabled
	st.NextFire = next
	return m.store.SaveSchedule(ctx, st)
}

// Get returns one schedule's state.
func (m *Module) Get(ctx context.Context, id ingestion.ScheduleID) (ingestion.ScheduleState, error) {
	return m.store.LoadSchedule(ctx, id)
}

// List returns schedules matching the filter.
func (m *Module) List(ctx context.Context, f ingestion.ScheduleFilter) ([]ingestion.ScheduleState, error) {
	return m.store.ListSchedules(ctx, f)
}

// Pause disables a schedule so the timer stops claiming it.
func (m *Module) Pause(ctx context.Context, id ingestion.ScheduleID) error {
	return m.setEnabled(ctx, id, false)
}

// Resume re-enables a schedule, recomputing its next fire from now.
func (m *Module) Resume(ctx context.Context, id ingestion.ScheduleID) error {
	st, err := m.store.LoadSchedule(ctx, id)
	if err != nil {
		return err
	}
	next, err := nextFire(st.Spec, time.Now())
	if err != nil {
		return err
	}
	st.Enabled = true
	st.NextFire = next
	return m.store.SaveSchedule(ctx, st)
}

// Delete removes a schedule.
func (m *Module) Delete(ctx context.Context, id ingestion.ScheduleID) error {
	return m.store.DeleteSchedule(ctx, id)
}

func (m *Module) setEnabled(ctx context.Context, id ingestion.ScheduleID, on bool) error {
	st, err := m.store.LoadSchedule(ctx, id)
	if err != nil {
		return err
	}
	st.Enabled = on
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
		if st.Spec.Overlap == ingestion.OverlapSkip && m.previousActive(ctx, st) {
			m.advance(ctx, st, now)
			continue
		}
		runID, err := runs.Submit(ctx, m.bus, m.ds, st.Spec.Request)
		if err != nil {
			if m.log != nil {
				m.log.Error("scheduler: submit", err, ingestion.Field{Key: "schedule", Value: string(st.ID)})
			}
			m.advance(ctx, st, now)
			continue
		}

		t := now
		st.LastFired = &t
		st.LastRun = runID
		st.LastStatus = ingestion.RunRequested
		if next, err := nextFire(st.Spec, now); err == nil {
			st.NextFire = next
		}
		if err := m.store.SaveSchedule(ctx, st); err != nil {
			return fired, err
		}

		_ = events.Emit(ctx, m.bus, events.ScheduleFired,
			events.Envelope{Tenant: st.Spec.Tenant, Run: runID, At: now}, events.ScheduleFiredEvent{})
		fired++
	}
	return fired, nil
}

// advance recomputes and persists a schedule's next fire without firing it.
func (m *Module) advance(ctx context.Context, st ingestion.ScheduleState, now time.Time) {
	if next, err := nextFire(st.Spec, now); err == nil {
		st.NextFire = next
		_ = m.store.SaveSchedule(ctx, st)
	}
}

// previousActive reports whether the schedule's last run is still requested or
// running — the condition OverlapSkip avoids stacking on.
func (m *Module) previousActive(ctx context.Context, st ingestion.ScheduleState) bool {
	if st.LastRun == "" {
		return false
	}
	prev, err := m.ds.LoadRun(ctx, st.LastRun)
	if err != nil {
		return false
	}
	return prev.Status == ingestion.RunRequested || prev.Status == ingestion.RunRunning
}

// nextFire computes the next fire time after `after` for the spec's cron in its
// timezone (UTC when unset).
func nextFire(spec ingestion.ScheduleSpec, after time.Time) (*time.Time, error) {
	sched, err := cron.Parse(spec.Cron)
	if err != nil {
		return nil, err
	}
	loc := time.UTC
	if spec.Timezone != "" {
		l, err := time.LoadLocation(spec.Timezone)
		if err != nil {
			return nil, fmt.Errorf("scheduler: timezone %q: %w", spec.Timezone, err)
		}
		loc = l
	}
	next, ok := sched.Next(after.In(loc))
	if !ok {
		return nil, fmt.Errorf("scheduler: cron %q has no next fire", spec.Cron)
	}
	return &next, nil
}

func newScheduleID() ingestion.ScheduleID {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return ingestion.ScheduleID("sched_" + hex.EncodeToString(b[:]))
}
