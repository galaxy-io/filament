package runs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
)

func TestPlanSignal(t *testing.T) {
	tests := []struct {
		name       string
		status     filament.RunStatus
		signal     filament.Signal
		to         filament.RunStatus
		transition bool
		ended      bool
		publish    bool
		worker     bool
		wantErr    error
	}{
		{name: "pause requested", status: filament.RunRequested, signal: filament.SignalPause, to: filament.RunPaused, transition: true},
		{name: "pause idempotent", status: filament.RunPaused, signal: filament.SignalPause},
		{name: "pause running", status: filament.RunRunning, signal: filament.SignalPause, publish: true, worker: true},
		{name: "resume paused", status: filament.RunPaused, signal: filament.SignalResume, to: filament.RunRequested, transition: true},
		{name: "resume partial", status: filament.RunPartial, signal: filament.SignalResume, to: filament.RunRequested, transition: true},
		{name: "resume requested repairs dispatch", status: filament.RunRequested, signal: filament.SignalResume, publish: true},
		{name: "cancel requested", status: filament.RunRequested, signal: filament.SignalCancel, to: filament.RunCanceled, transition: true, ended: true},
		{name: "cancel paused", status: filament.RunPaused, signal: filament.SignalCancel, to: filament.RunCanceled, transition: true, ended: true},
		{name: "cancel partial", status: filament.RunPartial, signal: filament.SignalCancel, to: filament.RunCanceled, transition: true, ended: true},
		{name: "cancel idempotent", status: filament.RunCanceled, signal: filament.SignalCancel},
		{name: "cancel running", status: filament.RunRunning, signal: filament.SignalCancel, publish: true, worker: true},
		{name: "cancel scheduled rejected", status: filament.RunScheduled, signal: filament.SignalCancel, wantErr: ErrSignalTransition},
		{name: "resume completed rejected", status: filament.RunCompleted, signal: filament.SignalResume, wantErr: ErrSignalTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planSignal(tt.status, tt.signal)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.to != tt.to || got.transition != tt.transition || got.ended != tt.ended || got.publish != tt.publish || got.worker != tt.worker {
				t.Fatalf("command = %#v", got)
			}
		})
	}
}

func TestPauseRejectsMixedCheckpointCoverage(t *testing.T) {
	state := filament.RunState{
		Run: "run", Tenant: "tenant", Status: filament.RunRunning,
		Request: filament.RunRequest{
			Resources: []string{"users", "audit"},
			IngestionTypes: map[string]filament.IngestionType{
				"users": filament.IngestionFullUpsert,
				"audit": filament.IngestionFullAppend,
			},
		},
	}
	if _, err := Signal(context.Background(), inproc.New(), sqlite.NewMemory(), state, filament.SignalPause); !errors.Is(err, ErrSignalTransition) {
		t.Fatalf("mixed pause error = %v, want transition error", err)
	}
}

func TestPlanTransitionPreservesOnlyCheckpointedProgress(t *testing.T) {
	tests := []struct {
		name      string
		status    filament.RunStatus
		ingestion filament.IngestionType
		preserve  bool
	}{
		{name: "paused resumable", status: filament.RunPaused, ingestion: filament.IngestionFullUpsert, preserve: true},
		{name: "paused restart", status: filament.RunPaused, ingestion: filament.IngestionFullReplace},
		{name: "partial progress is attempt-local", status: filament.RunPartial, ingestion: filament.IngestionFullUpsert},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := filament.RunState{Status: tt.status, Request: filament.RunRequest{
				Resources:      []string{"users"},
				IngestionTypes: map[string]filament.IngestionType{"users": tt.ingestion},
			}}
			got, ok := PlanTransition(state, filament.SignalResume)
			if !ok || got.From != tt.status || got.To != filament.RunRequested || !got.Options.ResetExecution {
				t.Fatalf("PlanTransition = %#v, %v", got, ok)
			}
			if got.Options.PreserveProgress != tt.preserve {
				t.Fatalf("PreserveProgress = %v, want %v", got.Options.PreserveProgress, tt.preserve)
			}
		})
	}
}

func TestPlanTransitionRejectsStatusTheCommandDoesNotApplyFrom(t *testing.T) {
	for _, tt := range []struct {
		status filament.RunStatus
		signal filament.Signal
	}{
		{filament.RunRunning, filament.SignalCancel},
		{filament.RunRunning, filament.SignalPause},
		{filament.RunCanceled, filament.SignalResume},
		{filament.RunPaused, filament.SignalPause},
		{filament.RunRequested, filament.SignalResume},
	} {
		if got, ok := PlanTransition(filament.RunState{Status: tt.status}, tt.signal); ok {
			t.Fatalf("PlanTransition(status %d, signal %d) = %#v, want no transition", tt.status, tt.signal, got)
		}
	}
}

func TestCancelStampsEndedAtOnlyForCancel(t *testing.T) {
	cancel, ok := PlanTransition(filament.RunState{Status: filament.RunRequested}, filament.SignalCancel)
	if !ok || cancel.To != filament.RunCanceled || !cancel.Options.Ended {
		t.Fatalf("cancel = %#v, %v; want canceled with ended_at", cancel, ok)
	}
	pause, ok := PlanTransition(filament.RunState{Status: filament.RunRequested}, filament.SignalPause)
	if !ok || pause.To != filament.RunPaused || pause.Options.Ended {
		t.Fatalf("pause = %#v, %v; want paused without ended_at", pause, ok)
	}
}

// Signal leaves run state to the tracker: it publishes the command and waits
// for the fold, and with no tracker folding it the store never changes.
func TestSignalPublishesCommandWithoutWritingRunState(t *testing.T) {
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "run", Tenant: "tenant", Status: filament.RunRequested, RequestedAt: time.Now()}
	if err := store.SaveRun(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	bus := inproc.New()
	sub, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := Signal(ctx, bus, store, state, filament.SignalCancel); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Signal with no tracker = %v, want the context's deadline", err)
	}
	select {
	case msg := <-sub.C():
		fact, err := events.Decode(msg)
		if err != nil || fact.Name != events.RunCancelCommanded.Name() {
			t.Fatalf("published %#v, %v; want %s", fact, err, events.RunCancelCommanded.Name())
		}
	case <-time.After(time.Second):
		t.Fatal("Signal published no command")
	}
	got, err := store.LoadRun(context.Background(), state.Tenant, state.Run)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != filament.RunRequested || got.EndedAt != nil {
		t.Fatalf("Signal wrote run state: status %d, ended %v", got.Status, got.EndedAt)
	}
}

// A command folded after another fact moved the run reports a conflict
// instead of success.
func TestSignalReportsCommandTheTrackerCouldNotApply(t *testing.T) {
	store := sqlite.NewMemory()
	state := filament.RunState{Run: "run", Tenant: "tenant", Status: filament.RunRequested, RequestedAt: time.Now()}
	if err := store.SaveRun(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	bus := inproc.New()
	sub, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	go func() {
		// The run starts before the tracker reaches the cancel.
		<-sub.C()
		started := state
		started.Status = filament.RunRunning
		_ = store.SaveRun(context.Background(), started)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Signal(ctx, bus, store, state, filament.SignalCancel); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("Signal = %v, want a version conflict", err)
	}
}

func TestSkippedOnlyWhenTheCommandCanNoLongerApply(t *testing.T) {
	for _, tt := range []struct {
		status filament.RunStatus
		signal filament.Signal
		want   bool
	}{
		{filament.RunPaused, filament.SignalCancel, false}, // a pause folded first, the cancel still applies
		{filament.RunCanceled, filament.SignalCancel, false},
		{filament.RunRunning, filament.SignalCancel, true},
		{filament.RunRunning, filament.SignalPause, true},
		{filament.RunCanceled, filament.SignalPause, true},
		{filament.RunRequested, filament.SignalResume, false},
		{filament.RunRunning, filament.SignalResume, false}, // resumed and dispatched
		{filament.RunCanceled, filament.SignalResume, true},
	} {
		if got := skipped(filament.RunState{Status: tt.status}, tt.signal); got != tt.want {
			t.Errorf("skipped(status %d, signal %d) = %v, want %v", tt.status, tt.signal, got, tt.want)
		}
	}
}
