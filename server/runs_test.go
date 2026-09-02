package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/identity"
)

type loadRunErrorStore struct {
	filament.DataStore
	err error
}

func TestSignalRunPauseResumeAndCancelStateMachine(t *testing.T) {
	ctx := identity.WithTenant(context.Background(), "tenant-1")
	store := memory.New()
	api := &Server{store: store, bus: inproc.New()}
	ended := time.Now()
	state := filament.RunState{
		Run: "run-1", Tenant: "tenant-1", Status: filament.RunRequested,
		StartedAt: time.Now().Add(-time.Minute), EndedAt: &ended, Records: 9, Bytes: 90,
		Resources: []filament.ResourceState{{Resource: "users", Status: filament.RunCompleted, Records: 9, Bytes: 90, Error: "old"}},
	}
	if err := store.SaveRun(ctx, state); err != nil {
		t.Fatal(err)
	}

	signal := func(value ingestionv1.RunSignal) error {
		_, err := api.SignalRun(ctx, connect.NewRequest(&ingestionv1.SignalRunRequest{
			RunId: "run-1", Signal: value,
		}))
		return err
	}
	if err := signal(ingestionv1.RunSignal_RUN_SIGNAL_PAUSE); err != nil {
		t.Fatalf("pause requested run: %v", err)
	}
	paused, _ := store.LoadRun(ctx, "tenant-1", "run-1")
	if paused.Status != filament.RunPaused {
		t.Fatalf("status after pause = %v", paused.Status)
	}
	if err := signal(ingestionv1.RunSignal_RUN_SIGNAL_RESUME); err != nil {
		t.Fatalf("resume paused run: %v", err)
	}
	resumed, _ := store.LoadRun(ctx, "tenant-1", "run-1")
	if resumed.Status != filament.RunRequested || !resumed.StartedAt.IsZero() || resumed.EndedAt != nil || resumed.Records != 0 || resumed.Error != "" {
		t.Fatalf("resumed state was not reset: %#v", resumed)
	}
	if len(resumed.Resources) != 1 || resumed.Resources[0].Status != filament.RunRequested || resumed.Resources[0].Records != 0 || resumed.Resources[0].Error != "" {
		t.Fatalf("resumed resources were not reset: %#v", resumed.Resources)
	}
	if err := signal(ingestionv1.RunSignal_RUN_SIGNAL_CANCEL); err != nil {
		t.Fatalf("cancel requested run: %v", err)
	}
	canceled, _ := store.LoadRun(ctx, "tenant-1", "run-1")
	if canceled.Status != filament.RunCanceled {
		t.Fatalf("status after cancel = %v", canceled.Status)
	}
}

func TestSignalRunPublishesRunningWorkerCommands(t *testing.T) {
	ctx := identity.WithTenant(context.Background(), "tenant-1")
	store := memory.New()
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-1", Tenant: "tenant-1", Status: filament.RunRunning}); err != nil {
		t.Fatal(err)
	}
	bus := inproc.New()
	sub, err := bus.Subscribe("ingestion.v1.run.tenant-1.run-1.*", eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	api := &Server{store: store, bus: bus}
	setStatus := func(status filament.RunStatus) error {
		state, err := store.LoadRun(ctx, "tenant-1", "run-1")
		if err != nil {
			return err
		}
		state.Status = status
		return store.SaveRun(ctx, state)
	}
	for _, test := range []struct {
		signal ingestionv1.RunSignal
		want   string
		ack    func() error
	}{
		{ingestionv1.RunSignal_RUN_SIGNAL_PAUSE, "run.pause_requested", func() error {
			if err := setStatus(filament.RunPaused); err != nil {
				return err
			}
			return events.Emit(ctx, bus, events.RunPaused, events.Envelope{
				Tenant: "tenant-1", Run: "run-1", At: time.Now(),
			}, events.RunPausedEvent{})
		}},
		{ingestionv1.RunSignal_RUN_SIGNAL_CANCEL, "run.cancel_requested", func() error {
			if err := setStatus(filament.RunCanceled); err != nil {
				return err
			}
			return events.Emit(ctx, bus, events.RunCanceled, events.Envelope{
				Tenant: "tenant-1", Run: "run-1", At: time.Now(),
			}, events.RunCanceledEvent{})
		}},
	} {
		if err := setStatus(filament.RunRunning); err != nil {
			t.Fatal(err)
		}
		errCh := make(chan error, 1)
		go func() {
			_, signalErr := api.SignalRun(ctx, connect.NewRequest(&ingestionv1.SignalRunRequest{RunId: "run-1", Signal: test.signal}))
			errCh <- signalErr
		}()
		deadline := time.After(time.Second)
		published := false
		for !published {
			select {
			case msg := <-sub.C():
				fact, decodeErr := events.Decode(msg)
				_ = msg.Ack()
				if decodeErr == nil && fact.Name == test.want {
					if err := test.ack(); err != nil {
						t.Fatal(err)
					}
					published = true
				}
			case <-deadline:
				t.Fatalf("signal %v did not publish %q", test.signal, test.want)
			}
		}
		if err := <-errCh; err != nil {
			t.Fatalf("signal %v: %v", test.signal, err)
		}
	}
	state, err := store.LoadRun(ctx, "tenant-1", "run-1")
	if err != nil || state.Status != filament.RunCanceled {
		t.Fatalf("worker acknowledgement was not persisted: %#v, %v", state, err)
	}
}

func TestSignalRunReportsWhenWorkerAlreadyFinished(t *testing.T) {
	ctx := identity.WithTenant(context.Background(), "tenant-1")
	store := memory.New()
	state := filament.RunState{Run: "run-1", Tenant: "tenant-1", Status: filament.RunRunning}
	if err := store.SaveRun(ctx, state); err != nil {
		t.Fatal(err)
	}
	bus := inproc.New()
	commands, err := bus.Subscribe(events.Subject(events.RunCancelRequested, state.Tenant, state.Run), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = commands.Close() }()
	api := &Server{store: store, bus: bus}
	errCh := make(chan error, 1)
	go func() {
		_, signalErr := api.SignalRun(ctx, connect.NewRequest(&ingestionv1.SignalRunRequest{
			RunId: "run-1", Signal: ingestionv1.RunSignal_RUN_SIGNAL_CANCEL,
		}))
		errCh <- signalErr
	}()
	select {
	case msg := <-commands.C():
		_ = msg.Ack()
	case <-time.After(time.Second):
		t.Fatal("cancel command was not published")
	}
	if err := events.Emit(ctx, bus, events.RunCompleted,
		events.Envelope{Tenant: state.Tenant, Run: state.Run, At: time.Now()}, events.RunCompletedEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("signal error = %v, want failed precondition", err)
	}
}

func (s loadRunErrorStore) LoadRun(context.Context, filament.TenantID, filament.RunID) (filament.RunState, error) {
	return filament.RunState{}, s.err
}

func TestLoadRunSnapshotOnlySuppressesNotFound(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("database unavailable")
	server := &Server{store: loadRunErrorStore{DataStore: memory.New(), err: storeErr}}
	if _, ok, err := server.loadRunSnapshot(context.Background(), "tenant-1", "run-1"); ok || !errors.Is(err, storeErr) {
		t.Fatalf("loadRunSnapshot() = (_, %v, %v), want (_, false, store error)", ok, err)
	}

	server.store = loadRunErrorStore{DataStore: memory.New(), err: filament.ErrNotFound}
	if _, ok, err := server.loadRunSnapshot(context.Background(), "tenant-1", "run-1"); ok || err != nil {
		t.Fatalf("loadRunSnapshot() = (_, %v, %v), want (_, false, nil)", ok, err)
	}
}
