package export

import (
	"fmt"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// PlanState validates a prior checkpoint or initializes an empty job set.
func (s *Runtime) PlanState(res manifest.Resource, cp filament.Checkpoint, inc bool) (RunState, error) {
	if cp == nil {
		return s.emptyExportRun(res, inc), nil
	}
	ks, ok := checkpoint.ParseKeyset(cp)
	mode := checkpoint.ModeKeyset
	if inc {
		mode = checkpoint.ModeIncremental
	}
	if !ok || ks.Mode != mode || len(ks.Shards) != 1 || len(ks.Shards[0].Key) != 1 || !slices.Equal(ks.Cols, []string{"export_state"}) || !checkpoint.RequiresReplay(cp) {
		return RunState{}, fmt.Errorf("export %s: incompatible checkpoint", res.Name)
	}
	return s.decodeExportRun(res, ks.Shards[0].Key[0], inc)
}

// PlanIncremental advances candidates from a committed cross-run checkpoint.
// An all-done cross-run checkpoint proves sink commit. Only here do completed
// job candidates become committed parent watermarks and permit fresh jobs.
// A committed pause may still contain pending jobs; retain that run's frozen set.
func (s *Runtime) PlanIncremental(res manifest.Resource, previous filament.Checkpoint) (filament.Checkpoint, error) {
	state, err := s.PlanState(res, previous, true)
	if err != nil {
		return nil, err
	}
	for _, key := range state.Selected {
		if state.Parents[key].Job.Phase != "done" {
			return Checkpoint(res.Name, state)
		}
	}
	for key, entry := range state.Parents {
		if entry.Job != nil && entry.Job.Phase == "done" && entry.Job.Candidate != "" {
			entry.Committed = entry.Job.Candidate
		}
		entry.Job = nil
		state.Parents[key] = entry
	}
	state.Initialized = false
	state.Selected = nil
	return Checkpoint(res.Name, state)
}

// Restore combines a durable plan with same-run job state, rejecting attempts
// whose committed watermarks no longer match. Run replays their artifacts.
func (s *Runtime) Restore(res manifest.Resource, cp, attempt filament.Checkpoint) (filament.Checkpoint, error) {
	baseline, err := s.PlanState(res, cp, true)
	if err != nil {
		return nil, err
	}
	state := baseline
	if saved := attempt; saved != nil {
		state, err = s.PlanState(res, saved, true)
		if err != nil {
			return nil, err
		}
		// Reject a stale attempt after a different run advanced the durable state.
		for key, entry := range state.Parents {
			if entry.Committed != baseline.Parents[key].Committed {
				return nil, fmt.Errorf("export %s attempt does not match the committed watermark", res.Name)
			}
		}
		for key, entry := range baseline.Parents {
			if entry.Committed != state.Parents[key].Committed {
				return nil, fmt.Errorf("export %s attempt is missing committed parent state", res.Name)
			}
		}
	}
	return Checkpoint(res.Name, state)
}
