package runner

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
)

type commitTestSource struct{ incrementalTestSource }

func (*commitTestSource) PlanResume(context.Context, []string, map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	return nil, nil
}

// commitFailSink completes every write, then fails Commit, recording Abort calls.
type commitFailSink struct {
	incrementalTestSink
	aborts atomic.Int32
}

func (s *commitFailSink) Commit(context.Context) error { return errors.New("commit boom") }
func (s *commitFailSink) Abort(context.Context) error  { s.aborts.Add(1); return nil }

type controlledTestSource struct {
	commitTestSource
	started chan struct{}
}

func (s *controlledTestSource) wait(ctx context.Context) error {
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}

func (s *controlledTestSource) Extract(ctx context.Context, _ filament.RecordSink, _ filament.ExtractOpts) error {
	return s.wait(ctx)
}

func (s *controlledTestSource) ExtractFrom(
	ctx context.Context,
	_ filament.RecordSink,
	_ filament.ExtractOpts,
	_ map[string]filament.Checkpoint,
) error {
	return s.wait(ctx)
}

type controlledTestSink struct {
	incrementalTestSink
	commits atomic.Int32
	aborts  atomic.Int32
}

func (s *controlledTestSink) Commit(context.Context) error { s.commits.Add(1); return nil }
func (s *controlledTestSink) Abort(context.Context) error  { s.aborts.Add(1); return nil }

type progressLoadErrorStore struct {
	filament.DataStore
}

func (progressLoadErrorStore) LoadRun(context.Context, filament.TenantID, filament.RunID) (filament.RunState, error) {
	return filament.RunState{}, errors.New("database unavailable")
}

type publishErrorBus struct{ eventbus.Bus }

func (publishErrorBus) Publish(context.Context, string, any) error {
	return errors.New("broker unavailable")
}

func TestRunOneDoesNotExecuteWithoutPublishedStart(t *testing.T) {
	base := inproc.New()
	defer func() { _ = base.Close() }()
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "r1", Tenant: "t1", Status: filament.RunRequested}
	if err := store.SaveRun(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	err := RunOne(context.Background(), Deps{
		Bus: publishErrorBus{Bus: base}, DataStore: store,
	}, filament.RunSpec{Tenant: state.Tenant, Run: state.Run})
	if err == nil || !strings.Contains(err.Error(), "publish run.started") {
		t.Fatalf("RunOne error = %v, want run.started publication failure", err)
	}
	got, err := store.LoadRun(context.Background(), state.Tenant, state.Run)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != filament.RunRequested {
		t.Fatalf("status = %v, want Requested", got.Status)
	}
}

// publishFailOnceBus fails the first publish only, the shape of a transient
// broker fault at worker start.
type publishFailOnceBus struct {
	eventbus.Bus
	failed atomic.Bool
}

func (b *publishFailOnceBus) Publish(ctx context.Context, subject string, payload any) error {
	if b.failed.CompareAndSwap(false, true) {
		return errors.New("broker unavailable")
	}
	return b.Bus.Publish(ctx, subject, payload)
}

func TestRunOneRetriesAfterTransientStartPublishFailure(t *testing.T) {
	base := inproc.New()
	defer func() { _ = base.Close() }()
	facts, err := base.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facts.Close() }()
	completed := make(chan struct{}, 1)
	obituary := make(chan string, 1)
	go func() {
		for msg := range facts.C() {
			f, decodeErr := events.Decode(msg)
			_ = msg.Ack()
			if decodeErr != nil {
				continue
			}
			switch d := f.Data.(type) {
			case events.RunCompletedEvent:
				completed <- struct{}{}
			case events.RunFailedEvent:
				obituary <- d.Error
			}
		}
	}()

	sources := registry.NewSources()
	sources.Register("test", func() filament.Source { return &commitTestSource{} })
	sinks := registry.NewSinks()
	sinks.Register("test-sink", func() filament.Sink { return &controlledTestSink{} })
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "r1", Tenant: "t1", Status: filament.RunRequested}
	if err := store.SaveRun(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	deps := Deps{Bus: &publishFailOnceBus{Bus: base}, DataStore: store, Sources: sources, Sinks: sinks}
	spec := filament.RunSpec{
		Tenant: state.Tenant, Run: state.Run,
		Source: filament.Ref{Connector: "test"}, Sink: filament.Ref{Connector: "test-sink"},
		Resources:      []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullUpsert},
	}

	err = RunOne(context.Background(), deps, spec)
	if err == nil || !strings.Contains(err.Error(), "publish run.started") {
		t.Fatalf("first RunOne error = %v, want run.started publication failure", err)
	}
	select {
	case msg := <-obituary:
		t.Fatalf("admission failure published run.failed %q; the retry could never execute", msg)
	case <-time.After(100 * time.Millisecond):
	}
	if got, _ := store.LoadRun(context.Background(), state.Tenant, state.Run); got.Status != filament.RunRequested {
		t.Fatalf("status after failed admission = %v, want Requested", got.Status)
	}

	if err := RunOne(context.Background(), deps, spec); err != nil {
		t.Fatalf("retry RunOne error = %v", err)
	}
	select {
	case <-completed:
	case <-time.After(5 * time.Second):
		t.Fatal("retry did not publish run.completed")
	}
}

func TestRunOneFailsWhenProgressCannotBeRestored(t *testing.T) {
	bus := inproc.New()
	facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facts.Close() }()
	RunOne(context.Background(), Deps{
		Bus: bus, DataStore: progressLoadErrorStore{DataStore: sqlite.NewMemory()},
	}, filament.RunSpec{Tenant: "tenant", Run: "run"})
	waitForFact(t, facts, events.RunFailed.Name())
}

func TestRunOneCommitFailureAborts(t *testing.T) {
	bus := inproc.New()
	facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facts.Close() }()

	failed := make(chan string, 1)
	resourceFailed := make(chan string, 1)
	go func() {
		for msg := range facts.C() {
			f, decodeErr := events.Decode(msg)
			_ = msg.Ack()
			if decodeErr != nil {
				continue
			}
			if d, ok := f.Data.(events.RunFailedEvent); ok {
				failed <- d.Error
				return
			}
			if d, ok := f.Data.(events.ResourceFailedEvent); ok && f.Resource == "users" {
				resourceFailed <- d.Error
			}
		}
	}()

	sink := &commitFailSink{}
	sources := registry.NewSources()
	sources.Register("test", func() filament.Source { return &commitTestSource{} })
	sinks := registry.NewSinks()
	sinks.Register("test-sink", func() filament.Sink { return sink })

	RunOne(context.Background(), Deps{
		Bus:       bus,
		DataStore: sqlite.NewMemory(),
		Sources:   sources,
		Sinks:     sinks,
	}, filament.RunSpec{
		Tenant:         "t1",
		Run:            "r1",
		Source:         filament.Ref{Connector: "test"},
		Sink:           filament.Ref{Connector: "test-sink"},
		Resources:      []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullUpsert},
	})

	select {
	case msg := <-failed:
		if !strings.Contains(msg, "commit") {
			t.Fatalf("run.failed error = %q, want commit failure", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no run.failed fact published")
	}
	select {
	case msg := <-resourceFailed:
		if !strings.Contains(msg, "commit") {
			t.Fatalf("resource.failed error = %q, want commit failure", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no resource.failed fact published")
	}
	if got := sink.aborts.Load(); got != 1 {
		t.Fatalf("Abort calls = %d, want 1", got)
	}
}

func TestRunOneCooperativeControl(t *testing.T) {
	tests := []struct {
		name        string
		cancel      bool
		ingestion   filament.IngestionType
		terminal    string
		wantCommits int32
		wantAborts  int32
	}{
		{
			name:      "pause checkpointed run commits drained progress",
			ingestion: filament.IngestionFullUpsert, terminal: events.RunPaused.Name(), wantCommits: 1,
		},
		{
			name:      "pause checkpoint-free run aborts partial output",
			ingestion: filament.IngestionFullReplace, terminal: events.RunPaused.Name(), wantAborts: 1,
		},
		{
			name: "cancel aborts", cancel: true,
			ingestion: filament.IngestionFullUpsert, terminal: events.RunCanceled.Name(), wantAborts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := inproc.New()
			facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = facts.Close() }()

			source := &controlledTestSource{started: make(chan struct{})}
			sink := &controlledTestSink{}
			sources := registry.NewSources()
			sources.Register("test", func() filament.Source { return source })
			sinks := registry.NewSinks()
			sinks.Register("test-sink", func() filament.Sink { return sink })
			done := make(chan struct{})
			go func() {
				defer close(done)
				RunOne(context.Background(), Deps{
					Bus: bus, DataStore: sqlite.NewMemory(), Sources: sources, Sinks: sinks,
				}, filament.RunSpec{
					Tenant: "t1", Run: "r1", Source: filament.Ref{Connector: "test"}, Sink: filament.Ref{Connector: "test-sink"},
					Resources: []string{"users"}, IngestionTypes: map[string]filament.IngestionType{"users": tt.ingestion},
				})
			}()

			select {
			case <-source.started:
			case <-time.After(5 * time.Second):
				t.Fatal("source did not start")
			}
			env := events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}
			if tt.cancel {
				err = events.Emit(context.Background(), bus, events.RunCancelRequested, env, events.RunCancelRequestedEvent{})
			} else {
				err = events.Emit(context.Background(), bus, events.RunPauseRequested, env, events.RunPauseRequestedEvent{})
			}
			if err != nil {
				t.Fatal(err)
			}
			waitForFact(t, facts, tt.terminal)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("controlled run did not stop")
			}
			if got := sink.commits.Load(); got != tt.wantCommits {
				t.Fatalf("Commit calls = %d, want %d", got, tt.wantCommits)
			}
			if got := sink.aborts.Load(); got != tt.wantAborts {
				t.Fatalf("Abort calls = %d, want %d", got, tt.wantAborts)
			}
		})
	}
}

func waitForFact(t *testing.T, sub eventbus.Subscription, name string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-sub.C():
			fact, err := events.Decode(msg)
			_ = msg.Ack()
			if err != nil {
				continue
			}
			if fact.Name == name {
				return
			}
			if _, failed := fact.Data.(events.RunFailedEvent); failed {
				t.Fatalf("run failed while waiting for %s: %#v", name, fact.Data)
			}
		case <-deadline:
			t.Fatalf("no %s fact published", name)
		}
	}
}

func TestEmitterSeedsResumedProgress(t *testing.T) {
	em := newEmitter(context.Background(), inproc.New(), nil, "tenant", "run")
	em.seedProgress(filament.RunState{
		Run: "run", Records: 125, Bytes: 500,
		Resources: []filament.ResourceState{{Resource: "users", Records: 125, Bytes: 500}},
	})
	if records, bytes := em.runTotals(); records != 125 || bytes != 500 {
		t.Fatalf("run totals = %d/%d, want 125/500", records, bytes)
	}
	if records, bytes := em.resourceTally("users"); records != 125 || bytes != 500 {
		t.Fatalf("resource totals = %d/%d, want 125/500", records, bytes)
	}
}

func TestSeedEmitterProgressIgnoresPartialAttemptCounters(t *testing.T) {
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "run", Status: filament.RunPartial, Records: 125, Bytes: 500}
	if err := store.SaveRun(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	em := newEmitter(context.Background(), inproc.New(), nil, "tenant", "run")
	if err := seedEmitterProgress(context.Background(), store, state.Tenant, state.Run, em); err != nil {
		t.Fatal(err)
	}
	if records, bytes := em.runTotals(); records != 0 || bytes != 0 {
		t.Fatalf("partial totals = %d/%d, want 0/0", records, bytes)
	}
}
