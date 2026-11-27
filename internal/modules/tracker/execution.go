package tracker

import (
	"context"
	"errors"

	"github.com/galaxy-io/filament"
)

type runKey struct {
	tenant filament.TenantID
	run    filament.RunID
}

// Bound the cache even for continuous runs, which bypass terminal folding.
const maxExecutionModes = 4096

// runInfo is what the tracker remembers of a stored run between its facts.
type runInfo struct {
	mode     filament.ExecutionMode
	pipeline string
}

func (m *Module) executionMode(ctx context.Context, tenant filament.TenantID, run filament.RunID) (filament.ExecutionMode, error) {
	info, _, err := m.runInfo(ctx, tenant, run)
	return info.mode, err
}

// runInfo returns the run's execution mode and pipeline, reading the store
// only on the run's first fact in this process. known is false for a run the
// store does not hold yet, whose mode is then assumed bounded.
func (m *Module) runInfo(ctx context.Context, tenant filament.TenantID, run filament.RunID) (info runInfo, known bool, err error) {
	key := runKey{tenant, run}
	m.mu.Lock()
	info, ok := m.modes[key]
	m.mu.Unlock()
	if ok {
		return info, true, nil
	}
	state, err := m.ds.LoadRun(ctx, tenant, run)
	if errors.Is(err, filament.ErrNotFound) {
		// Preserve legacy folding for facts that precede their run. Never cache a
		// guessed mode; the actual run may arrive before the next fact.
		return runInfo{mode: filament.ExecutionBounded}, false, nil
	}
	if err != nil {
		return runInfo{}, false, err
	}
	info = runInfo{mode: state.Request.Options.Execution.Normalize(), pipeline: state.Request.PipelineID}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.modes == nil {
		m.modes = make(map[runKey]runInfo)
	}
	if len(m.modes) >= maxExecutionModes {
		for old := range m.modes {
			delete(m.modes, old)
			break
		}
	}
	m.modes[key] = info
	return info, true, nil
}
