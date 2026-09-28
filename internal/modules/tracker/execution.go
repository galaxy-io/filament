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

func (m *Module) executionMode(ctx context.Context, tenant filament.TenantID, run filament.RunID) (filament.ExecutionMode, error) {
	key := runKey{tenant, run}
	m.mu.Lock()
	mode, ok := m.modes[key]
	m.mu.Unlock()
	if ok {
		return mode, nil
	}
	state, err := m.ds.LoadRun(ctx, tenant, run)
	if errors.Is(err, filament.ErrNotFound) {
		// Preserve legacy folding for facts that precede their run. Never cache a
		// guessed mode; the actual run may arrive before the next fact.
		return filament.ExecutionBounded, nil
	}
	if err != nil {
		return "", err
	}
	mode = state.Request.Options.Execution.Normalize()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.modes == nil {
		m.modes = make(map[runKey]filament.ExecutionMode)
	}
	if len(m.modes) >= maxExecutionModes {
		for old := range m.modes {
			delete(m.modes, old)
			break
		}
	}
	m.modes[key] = mode
	return mode, nil
}
