package tracker

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	runcommands "github.com/galaxy-io/filament/internal/runs"
	"github.com/galaxy-io/filament/module"
)

// signalStore is what the signal path and the tracker share in production.
type signalStore interface {
	filament.DataStore
	filament.RunTransitionStore
}

// loadHook runs afterLoad once, after the tracker's first LoadRun returns and
// before the fold that called it saves. It places a signal inside the
// tracker's read-modify-write without goroutine timing.
type loadHook struct {
	signalStore
	afterLoad func()
}

func (s *loadHook) LoadRun(ctx context.Context, tenant filament.TenantID, run filament.RunID) (filament.RunState, error) {
	state, err := s.signalStore.LoadRun(ctx, tenant, run)
	if hook := s.afterLoad; err == nil && hook != nil {
		s.afterLoad = nil
		hook()
	}
	return state, err
}

func openSignalStore(t *testing.T, backend string) signalStore {
	t.Helper()
	if backend == "sqlite" {
		return sqlite.NewMemory()
	}
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FILAMENT_TEST_POSTGRES_DSN not set; skipping Postgres signal fold test")
	}
	db, err := postgres.NewSQLDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := postgres.Migrate(db); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgres.New(pool)
}

// signalHarness runs the tracker's fold on the test goroutine, the way its
// single durable consumer does in production, while Signal runs elsewhere.
type signalHarness struct {
	t   *testing.T
	ctx context.Context
	bus *inproc.Bus
	sub eventbus.Subscription
	m   *Module
	// returned runs the moment Signal returns, before later facts fold.
	returned func(error)
}

// newSignalHarness mounts a tracker over store on a fresh bus and subscribes
// to every fact the bus carries.
func newSignalHarness(t *testing.T, ctx context.Context, store filament.DataStore) *signalHarness {
	t.Helper()
	bus := inproc.New()
	sub, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	m := New()
	if err := m.Mount(ctx, module.Deps{DataStore: store, Bus: bus}); err != nil {
		t.Fatal(err)
	}
	return &signalHarness{t: t, ctx: ctx, bus: bus, sub: sub, m: m}
}

func (h *signalHarness) start(state filament.RunState, signal filament.Signal) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := runcommands.Signal(h.ctx, h.bus, state, signal)
		done <- err
	}()
	return done
}

func (h *signalHarness) fold(msg eventbus.Message) {
	h.t.Helper()
	if err := h.m.onFact(h.ctx, msg); err != nil {
		h.t.Fatalf("fold %s: %v", msg.Subject(), err)
	}
}

// await folds facts until Signal returns, then folds what was published after.
func (h *signalHarness) await(done <-chan error) error {
	h.t.Helper()
	var err error
	for waiting := true; waiting; {
		select {
		case err = <-done:
			waiting = false
			if h.returned != nil {
				h.returned(err)
			}
		case msg := <-h.sub.C():
			h.fold(msg)
		case <-h.ctx.Done():
			h.t.Fatal("signal did not return")
		}
	}
	for {
		select {
		case msg := <-h.sub.C():
			h.fold(msg)
		case <-time.After(50 * time.Millisecond):
			return err
		}
	}
}

// TestSignalSurvivesConcurrentTrackerFold is the regression test for #596. A
// tracker fold that loaded the run before a pause, resume, or cancel must not
// restore the old status when it saves. Each transition runs with the signal
// before the fold's load, between its load and save, and after its save.
func TestSignalSurvivesConcurrentTrackerFold(t *testing.T) {
	cases := []struct {
		name   string
		from   filament.RunStatus
		to     filament.RunStatus
		signal filament.Signal
	}{
		{"pause_requested", filament.RunRequested, filament.RunPaused, filament.SignalPause},
		{"resume_paused", filament.RunPaused, filament.RunRequested, filament.SignalResume},
		{"resume_partial", filament.RunPartial, filament.RunRequested, filament.SignalResume},
		{"cancel_requested", filament.RunRequested, filament.RunCanceled, filament.SignalCancel},
		{"cancel_paused", filament.RunPaused, filament.RunCanceled, filament.SignalCancel},
		{"cancel_partial", filament.RunPartial, filament.RunCanceled, filament.SignalCancel},
	}
	for _, backend := range []string{"sqlite", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			store := openSignalStore(t, backend)
			for _, tc := range cases {
				for _, placement := range []string{"before_load", "during_load_save", "after_save"} {
					t.Run(tc.name+"/"+placement, func(t *testing.T) {
						ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
						defer cancel()
						tenant := filament.TenantID(uuid.NewString())
						run := filament.RunID(uuid.NewString())
						if err := store.EnsureTenant(ctx, tenant, "signal fold test"); err != nil {
							t.Fatal(err)
						}
						initial := filament.RunState{
							Tenant: tenant, Run: run, Status: tc.from, Records: 7,
							RequestedAt: time.Now(), Request: filament.RunRequest{Tenant: tenant},
						}
						if err := store.SaveRun(ctx, initial); err != nil {
							t.Fatal(err)
						}

						wrapped := &loadHook{signalStore: store}
						h := newSignalHarness(t, ctx, wrapped)
						m, sub := h.m, h.sub
						// A successful signal is acknowledged to the caller, so
						// the stored status must already be the target then.
						h.returned = func(err error) {
							if err != nil {
								return
							}
							if got, err := store.LoadRun(ctx, tenant, run); err != nil {
								t.Error(err)
							} else if got.Status != tc.to {
								t.Errorf("stored status = %d when Signal returned, want %d", got.Status, tc.to)
							}
						}

						var signalErr error
						if placement == "before_load" {
							signalErr = h.await(h.start(initial, tc.signal))
						}
						// The signal is published while the fold holds a stale
						// copy of the run. Facts it puts on the bus are folded
						// after the fold saves, in stream order.
						var during <-chan error
						var queued []eventbus.Message
						var early *error // Signal returned inside the fold
						if placement == "during_load_save" {
							wrapped.afterLoad = func() {
								during = h.start(initial, tc.signal)
								select {
								case msg := <-sub.C():
									queued = append(queued, msg)
								case <-ctx.Done():
									t.Error("signal published nothing")
									return
								}
								// A Signal that writes the store itself returns
								// here, before the fold saves. One that waits for
								// the tracker cannot return until it folds.
								select {
								case err := <-during:
									early = &err
								case <-time.After(100 * time.Millisecond):
								}
							}
						}
						if err := m.mutate(ctx, events.Envelope{Tenant: tenant, Run: run}, func(r *filament.RunState) {
							r.Records++
						}); err != nil {
							t.Fatal(err)
						}
						if placement == "during_load_save" {
							if early != nil {
								h.returned(*early)
								signalErr = *early
							}
							for _, msg := range queued {
								h.fold(msg)
							}
							if early == nil {
								signalErr = h.await(during)
							}
						}
						if placement == "after_save" {
							signalErr = h.await(h.start(initial, tc.signal))
						}

						if signalErr != nil {
							t.Fatalf("Signal: %v", signalErr)
						}
						final, err := store.LoadRun(ctx, tenant, run)
						if err != nil {
							t.Fatal(err)
						}
						if final.Status != tc.to {
							t.Fatalf("stored status = %d after signal returned, want %d", final.Status, tc.to)
						}
						if tc.signal != filament.SignalResume && final.Records != 8 {
							t.Fatalf("records = %d, want the fold's increment kept (8)", final.Records)
						}
					})
				}
			}
		})
	}
}

// foldSignal runs Signal against a tracker that folds every fact it publishes.
func foldSignal(t *testing.T, store signalStore, state filament.RunState, signal filament.Signal) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	h := newSignalHarness(t, ctx, store)
	return h.await(h.start(state, signal))
}

func TestFoldedCancelStampsEndedAt(t *testing.T) {
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "run-cancel", Tenant: "tenant", Status: filament.RunRequested, RequestedAt: time.Now()}
	if err := store.SaveRun(t.Context(), state); err != nil {
		t.Fatal(err)
	}
	if err := foldSignal(t, store, state, filament.SignalCancel); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadRun(t.Context(), state.Tenant, state.Run)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != filament.RunCanceled || got.EndedAt == nil || !got.StartedAt.IsZero() {
		t.Fatalf("cancel = status %d, ended %v, started %v; want canceled with ended_at and no start", got.Status, got.EndedAt, got.StartedAt)
	}
}

func TestFoldedResumePreservesOnlyCheckpointedProgress(t *testing.T) {
	tests := []struct {
		name        string
		status      filament.RunStatus
		ingestion   filament.IngestionType
		wantRecords int64
	}{
		{name: "paused resumable", status: filament.RunPaused, ingestion: filament.IngestionFullUpsert, wantRecords: 125},
		{name: "paused restart", status: filament.RunPaused, ingestion: filament.IngestionFullReplace, wantRecords: 0},
		{name: "partial progress is attempt-local", status: filament.RunPartial, ingestion: filament.IngestionFullUpsert, wantRecords: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := sqlite.NewMemory()
			state := filament.RunState{
				Run: "run", Tenant: "tenant", Status: tt.status, Records: 125, Bytes: 500,
				Request: filament.RunRequest{
					Resources:      []string{"users"},
					IngestionTypes: map[string]filament.IngestionType{"users": tt.ingestion},
				},
				Resources: []filament.ResourceState{{Resource: "users", Status: filament.RunRunning, Records: 125, Bytes: 500}},
			}
			if err := store.SaveRun(t.Context(), state); err != nil {
				t.Fatal(err)
			}
			if err := foldSignal(t, store, state, filament.SignalResume); err != nil {
				t.Fatal(err)
			}
			got, err := store.LoadRun(t.Context(), state.Tenant, state.Run)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != filament.RunRequested {
				t.Fatalf("status = %d, want requested", got.Status)
			}
			if got.Records != tt.wantRecords || got.Resources[0].Records != tt.wantRecords {
				t.Fatalf("resume records = run %d, resource %d; want %d", got.Records, got.Resources[0].Records, tt.wantRecords)
			}
		})
	}
}

// failingBus fails the next publish of one event, then delivers normally.
type failingBus struct {
	*inproc.Bus
	fail string
}

func (b *failingBus) Publish(ctx context.Context, subject string, payload any) error {
	if f, ok := payload.(events.Fact); ok && f.Name == b.fail {
		b.fail = ""
		return errors.New("bus unavailable")
	}
	return b.Bus.Publish(ctx, subject, payload)
}

// A resume whose run.requested could not be published must still announce it
// when the command is redelivered, though the run is already requested and
// the redelivered command no longer plans a transition.
func TestCommandOutcomeIsPublishedOnRedelivery(t *testing.T) {
	ctx := t.Context()
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "run", Tenant: "tenant", Status: filament.RunPaused, RequestedAt: time.Now()}
	if err := store.SaveRun(ctx, state); err != nil {
		t.Fatal(err)
	}
	bus := &failingBus{Bus: inproc.New(), fail: events.RunRequested.Name()}
	requested, err := bus.Subscribe(events.SubjectPattern(events.RunRequested), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = requested.Close() }()
	m := New()
	if err := m.Mount(ctx, module.Deps{DataStore: store, Bus: bus}); err != nil {
		t.Fatal(err)
	}
	command := fakeMsg{fact: events.NewFact(events.RunResumeCommanded,
		events.Envelope{Tenant: state.Tenant, Run: state.Run, At: time.Now()}, events.RunResumeCommandedEvent{}), seq: 1}
	if err := m.onFact(ctx, command); err == nil {
		t.Fatal("fold succeeded though run.requested was not published")
	}
	if got, _ := store.LoadRun(ctx, state.Tenant, state.Run); got.Status != filament.RunRequested {
		t.Fatalf("status = %d, want requested after the fold", got.Status)
	}
	if err := m.onFact(ctx, command); err != nil {
		t.Fatalf("redelivered fold: %v", err)
	}
	select {
	case <-requested.C():
	case <-time.After(time.Second):
		t.Fatal("run.requested was not published on redelivery")
	}
}

// The tracker plans from the status it loads, so a command folded after the
// run moved on changes nothing.
func TestCommandFoldedAfterRunStartedIsIgnored(t *testing.T) {
	m, ds := mounted(t)
	env := events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}
	deliver(t, m, 1, events.RunStarted, env, events.RunStartedEvent{})
	deliver(t, m, 2, events.RunCancelCommanded, env, events.RunCancelCommandedEvent{})
	got, err := ds.LoadRun(t.Context(), "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != filament.RunRunning || got.EndedAt != nil {
		t.Fatalf("status %d, ended %v; want the running run untouched", got.Status, got.EndedAt)
	}
}

func mountedWithBus(t *testing.T, ds filament.DataStore) (*Module, *inproc.Bus) {
	t.Helper()
	bus := inproc.New()
	m := New()
	if err := m.Mount(t.Context(), module.Deps{DataStore: ds, Bus: bus}); err != nil {
		t.Fatal(err)
	}
	return m, bus
}

func awaitFact(t *testing.T, sub eventbus.Subscription, name string) events.Fact {
	t.Helper()
	for {
		select {
		case msg := <-sub.C():
			if fact, err := events.Decode(msg); err == nil && fact.Name == name {
				return fact
			}
		case <-time.After(time.Second):
			t.Fatalf("%s was not published", name)
		}
	}
}

// A pause outcome folded after a resume already moved the run must not pause
// it again. That ordering arises when a second signal's command reaches the
// stream before the first command's outcome.
func TestStaleCommandedOutcomeDoesNotRestoreStatus(t *testing.T) {
	m, ds := mounted(t)
	env := events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}
	if err := ds.SaveRun(t.Context(), filament.RunState{Tenant: "t1", Run: "r1", Status: filament.RunRequested, RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	deliver(t, m, 1, events.RunPauseCommanded, env, events.RunPauseCommandedEvent{})
	deliver(t, m, 2, events.RunResumeCommanded, env, events.RunResumeCommandedEvent{})
	deliver(t, m, 3, events.RunPaused, env, events.RunPausedEvent{Commanded: true})
	got, err := ds.LoadRun(t.Context(), "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != filament.RunRequested || got.EndedAt != nil {
		t.Fatalf("status %d, ended %v; want the resumed run left requested", got.Status, got.EndedAt)
	}
}

// A tracker that restarted after storing a pause, and so lost the outcome it
// had not published, republishes it when the command is redelivered.
func TestRedeliveredCommandRepublishesOutcomeAfterRestart(t *testing.T) {
	store := sqlite.NewMemory()
	if err := store.SaveRun(t.Context(), filament.RunState{Tenant: "t1", Run: "r1", Status: filament.RunPaused, RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m, bus := mountedWithBus(t, store)
	sub, err := bus.Subscribe(events.SubjectPattern(events.RunPaused), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	at := time.Now()
	deliver(t, m, 1, events.RunPauseCommanded, events.Envelope{Tenant: "t1", Run: "r1", At: at}, events.RunPauseCommandedEvent{})
	fact := awaitFact(t, sub, events.RunPaused.Name())
	if d, ok := fact.Data.(events.RunPausedEvent); !ok || !d.Commanded || !fact.At.Equal(at) {
		t.Fatalf("outcome = %#v, want a commanded run.paused with the command's emit time", fact)
	}
}

// A worker admitted before a cancel folded still runs. Its run.started must
// ask it to stop instead of leaving it running unseen.
func TestRunStartedAfterCancelAsksWorkerToStop(t *testing.T) {
	for _, tt := range []struct {
		status  filament.RunStatus
		request string
	}{
		{filament.RunCanceled, events.RunCancelRequested.Name()},
		{filament.RunPaused, events.RunPauseRequested.Name()},
	} {
		store := sqlite.NewMemory()
		if err := store.SaveRun(t.Context(), filament.RunState{Tenant: "t1", Run: "r1", Status: tt.status, RequestedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		m, bus := mountedWithBus(t, store)
		sub, err := bus.Subscribe(events.RunPattern("t1", "r1"), eventbus.SubOpts{})
		if err != nil {
			t.Fatal(err)
		}
		deliver(t, m, 1, events.RunStarted, events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}, events.RunStartedEvent{})
		awaitFact(t, sub, tt.request)
		_ = sub.Close()
		got, err := store.LoadRun(t.Context(), "t1", "r1")
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != filament.RunRunning {
			t.Fatalf("status %d after run.started, want running until the worker acknowledges", got.Status)
		}
	}
}

// noTransitions hides the RunTransitionStore capability.
type noTransitions struct{ filament.DataStore }

// A store that cannot apply transitions must not stall the tracker: the
// command is acked and the run is left alone.
func TestCommandIgnoredWithoutTransitionStore(t *testing.T) {
	store := sqlite.NewMemory()
	if err := store.SaveRun(t.Context(), filament.RunState{Tenant: "t1", Run: "r1", Status: filament.RunRequested, RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.Mount(t.Context(), module.Deps{DataStore: noTransitions{store}}); err != nil {
		t.Fatal(err)
	}
	deliver(t, m, 1, events.RunCancelCommanded, events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}, events.RunCancelCommandedEvent{})
	if got, _ := store.LoadRun(t.Context(), "t1", "r1"); got.Status != filament.RunRequested {
		t.Fatalf("status %d, want requested", got.Status)
	}
}

// A skipped command tells its caller so: the skip carries the command's name,
// its emit time, and the status that ruled it out.
func TestSkippedCommandPublishesItsDecision(t *testing.T) {
	store := sqlite.NewMemory()
	if err := store.SaveRun(t.Context(), filament.RunState{Tenant: "t1", Run: "r1", Status: filament.RunRunning, RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m, bus := mountedWithBus(t, store)
	sub, err := bus.Subscribe(events.SubjectPattern(events.RunCommandSkipped), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	at := time.Now()
	deliver(t, m, 1, events.RunCancelCommanded, events.Envelope{Tenant: "t1", Run: "r1", At: at}, events.RunCancelCommandedEvent{})
	fact := awaitFact(t, sub, events.RunCommandSkipped.Name())
	skip, ok := fact.Data.(events.RunCommandSkippedEvent)
	if !ok || skip.Command != events.RunCancelCommanded.Name() || skip.Status != filament.RunRunning || !fact.At.Equal(at) {
		t.Fatalf("skip = %#v, want the cancel's skip at status running with its emit time", fact)
	}
	if got, _ := store.LoadRun(t.Context(), "t1", "r1"); got.Status != filament.RunRunning {
		t.Fatalf("status %d, want running", got.Status)
	}
}

// A tracker that restarted after storing a resume, and so lost the
// run.requested it had not published, cannot tell that resume from another
// caller's. It reports a skip at requested instead of dispatching twice.
func TestRedeliveredResumeAfterRestartReportsSkip(t *testing.T) {
	store := sqlite.NewMemory()
	if err := store.SaveRun(t.Context(), filament.RunState{Tenant: "t1", Run: "r1", Status: filament.RunRequested, RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m, bus := mountedWithBus(t, store)
	sub, err := bus.Subscribe(events.RunPattern("t1", "r1"), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	deliver(t, m, 1, events.RunResumeCommanded, events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}, events.RunResumeCommandedEvent{})
	fact := awaitFact(t, sub, events.RunCommandSkipped.Name())
	if skip := fact.Data.(events.RunCommandSkippedEvent); skip.Status != filament.RunRequested {
		t.Fatalf("skip status %d, want requested", skip.Status)
	}
	select {
	case msg := <-sub.C():
		if f, _ := events.Decode(msg); f.Name == events.RunRequested.Name() {
			t.Fatal("redelivered resume published run.requested, which could dispatch the run twice")
		}
	case <-time.After(100 * time.Millisecond):
	}
}
