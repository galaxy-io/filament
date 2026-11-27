package main

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/scheduler"
)

func TestControlModulesMountsReaperUnderWorkerDispatch(t *testing.T) {
	t.Setenv("REAPER_INTERVAL_SECONDS", "1")
	t.Setenv("REAPER_STALE_AFTER_SECONDS", "1")

	store := sqlite.NewMemory()
	sched := scheduler.New(store)
	mods, reap := controlModules(sched, nil)
	if reap == nil {
		t.Fatal("controlModules(sched, nil) returned nil reaper under worker dispatch")
	}
	found := false
	for _, m := range mods {
		if m.Name() == "reaper" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("reaper module missing from controlModules slice under worker dispatch")
	}

	// Verify end-to-end: under worker dispatch (nil dispatcher), a stale Running
	// run from a SIGKILLed worker is reaped into RunFailed while a queued
	// Requested run is left untouched.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := inproc.New()
	h, err := boot.Mount(ctx, boot.Deps{Store: store}, bus, mods...)
	if err != nil {
		t.Fatalf("mount host: %v", err)
	}
	defer func() { _ = h.Close() }()

	if err := store.SaveRun(ctx, filament.RunState{
		Run:     "run-dead-worker",
		Tenant:  "tenant-1",
		Status:  filament.RunRunning,
		Request: filament.RunRequest{PipelineID: "pipe-cdc"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(ctx, filament.RunState{
		Run:     "run-queued",
		Tenant:  "tenant-1",
		Status:  filament.RunRequested,
		Request: filament.RunRequest{PipelineID: "pipe-queued"},
	}); err != nil {
		t.Fatal(err)
	}

	sub, err := bus.Subscribe(events.RunPattern("tenant-1", "run-dead-worker"), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	time.Sleep(1100 * time.Millisecond)
	reap.Start(ctx)

	select {
	case msg := <-sub.C():
		fact, decErr := events.Decode(msg)
		_ = msg.Ack()
		if decErr != nil || fact.Name != events.RunFailed.Name() {
			t.Fatalf("fact = %#v, err = %v; want run.failed", fact, decErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-mode reaper did not fail stale Running run")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		reapedState, err := store.LoadRun(ctx, "tenant-1", "run-dead-worker")
		if err != nil {
			t.Fatal(err)
		}
		if reapedState.Status == filament.RunFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status after reaper = %v, want RunFailed", reapedState.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	queuedState, err := store.LoadRun(ctx, "tenant-1", "run-queued")
	if err != nil {
		t.Fatal(err)
	}
	if queuedState.Status != filament.RunRequested {
		t.Fatalf("queued status = %v, want RunRequested (untouched without workload probe)", queuedState.Status)
	}
}
