package runs

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus/inproc"
)

func TestPlanSignal(t *testing.T) {
	tests := []struct {
		name       string
		status     filament.RunStatus
		signal     filament.Signal
		to         filament.RunStatus
		transition bool
		publish    bool
		worker     bool
		wantErr    error
	}{
		{name: "pause requested", status: filament.RunRequested, signal: filament.SignalPause, to: filament.RunPaused, transition: true, publish: true},
		{name: "pause idempotent", status: filament.RunPaused, signal: filament.SignalPause},
		{name: "pause running", status: filament.RunRunning, signal: filament.SignalPause, publish: true, worker: true},
		{name: "resume paused", status: filament.RunPaused, signal: filament.SignalResume, to: filament.RunRequested, transition: true, publish: true},
		{name: "resume partial", status: filament.RunPartial, signal: filament.SignalResume, to: filament.RunRequested, transition: true, publish: true},
		{name: "resume requested repairs dispatch", status: filament.RunRequested, signal: filament.SignalResume, publish: true},
		{name: "cancel requested", status: filament.RunRequested, signal: filament.SignalCancel, to: filament.RunCanceled, transition: true, publish: true},
		{name: "cancel idempotent", status: filament.RunCanceled, signal: filament.SignalCancel},
		{name: "cancel running", status: filament.RunRunning, signal: filament.SignalCancel, publish: true, worker: true},
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
			if got.to != tt.to || got.transition != tt.transition || got.publish != tt.publish || got.worker != tt.worker {
				t.Fatalf("command = %#v", got)
			}
		})
	}
}

func TestResumePreservesOnlyCheckpointedProgress(t *testing.T) {
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
			ctx := context.Background()
			store := memory.New()
			state := filament.RunState{
				Run: "run", Tenant: "tenant", Status: tt.status, Records: 125, Bytes: 500,
				Request: filament.RunRequest{
					Resources:      []string{"users"},
					IngestionTypes: map[string]filament.IngestionType{"users": tt.ingestion},
				},
				Resources: []filament.ResourceState{{Resource: "users", Status: filament.RunRunning, Records: 125, Bytes: 500}},
			}
			if err := store.SaveRun(ctx, state); err != nil {
				t.Fatal(err)
			}
			if _, err := Signal(ctx, inproc.New(), store, state, filament.SignalResume); err != nil {
				t.Fatal(err)
			}
			got, err := store.LoadRun(ctx, state.Run)
			if err != nil {
				t.Fatal(err)
			}
			if got.Records != tt.wantRecords || got.Resources[0].Records != tt.wantRecords {
				t.Fatalf("resume records = run %d, resource %d; want %d", got.Records, got.Resources[0].Records, tt.wantRecords)
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
	if _, err := Signal(context.Background(), inproc.New(), memory.New(), state, filament.SignalPause); !errors.Is(err, ErrSignalTransition) {
		t.Fatalf("mixed pause error = %v, want transition error", err)
	}
}
