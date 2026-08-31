package reaper

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestDeadNeedsDispatchEvidenceForRequested(t *testing.T) {
	for _, tt := range []struct {
		status   filament.RunStatus
		workload filament.Workload
		want     bool
	}{
		{filament.RunRunning, filament.WorkloadAbsent, true},
		{filament.RunRunning, filament.WorkloadActive, false},
		{filament.RunRunning, filament.WorkloadFinished, true},
		{filament.RunRequested, filament.WorkloadAbsent, false},
		{filament.RunRequested, filament.WorkloadActive, false},
		{filament.RunRequested, filament.WorkloadFinished, true},
		{filament.RunPartial, filament.WorkloadFinished, false},
	} {
		if got := dead(tt.status, tt.workload); got != tt.want {
			t.Errorf("dead(%v, %v) = %v, want %v", tt.status, tt.workload, got, tt.want)
		}
	}
}
