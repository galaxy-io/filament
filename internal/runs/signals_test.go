package runs

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
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
