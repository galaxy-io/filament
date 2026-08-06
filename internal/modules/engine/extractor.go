package engine

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/galaxy-io/filament"
)

type extractorFunc func(context.Context, filament.RecordSink, filament.ExtractOpts) error

func (m *Module) resolveExtractor(ctx context.Context, src filament.Source, spec filament.RunSpec, plan filament.IngestionPlan) (extractorFunc, error) {
	if plan.Type == filament.IngestionCDC {
		changes, ok := src.(filament.ChangeSource)
		if !ok {
			return nil, fmt.Errorf("source %q does not support CDC extraction", spec.Source.Provider)
		}
		checkpoints, err := m.loadChangeCheckpoints(ctx, spec)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			return changes.ExtractChanges(ctx, sink, filament.ChangeExtractOpts{
				Resources:   opts.Resources,
				Checkpoints: checkpoints,
				Limit:       opts.Limit,
			})
		}, nil
	}
	if isResumableRun(plan) {
		resumable, ok := src.(filament.Resumable)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Provider)
		}
		incremental := plan.SourcePolicy.Mode == filament.ModeIncremental
		prev := make(map[string]filament.Checkpoint, len(spec.Resources))
		for _, resource := range spec.Resources {
			var cp filament.Checkpoint
			var err error
			if incremental {
				key, valid := spec.ResourceCheckpointKey(resource)
				if !valid {
					return nil, fmt.Errorf("incremental resource %q requires a versioned pipeline route", resource)
				}
				var state filament.ResourceCheckpointState
				state, err = m.ds.LoadResourceCheckpoint(ctx, key)
				cp = state.Checkpoint
			} else {
				cp, err = m.ds.LoadCheckpoint(ctx, spec.Run, resource)
			}
			if err == nil {
				prev[resource] = cp
			} else if err != nil && !errors.Is(err, filament.ErrNotFound) {
				return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
			}
		}
		var resumePlan map[string]filament.Checkpoint
		var err error
		if incremental {
			planner, ok := src.(filament.IncrementalPlanner)
			if !ok {
				return nil, fmt.Errorf("source %q does not support incremental planning", spec.Source.Provider)
			}
			resumePlan, err = planner.PlanIncremental(ctx, spec.Resources, prev, spec.CursorConfigs)
		} else {
			planner, ok := src.(filament.ResumePlanner)
			if !ok {
				return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Provider)
			}
			resumePlan, err = planner.PlanResume(ctx, spec.Resources, prev)
		}
		if err != nil {
			return nil, fmt.Errorf("plan resume: %w", err)
		}
		for _, cp := range resumePlan {
			if cp == nil {
				continue
			}
			var saveErr error
			if incremental {
				key, _ := spec.ResourceCheckpointKey(cp.Resource())
				saveErr = m.ds.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: spec.Run, Checkpoint: cp})
			} else {
				saveErr = m.ds.SaveCheckpoint(ctx, spec.Run, cp)
			}
			if saveErr != nil {
				return nil, fmt.Errorf("seed checkpoint %q: %w", cp.Resource(), saveErr)
			}
		}
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
		}, nil
	}
	return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
		return src.Extract(ctx, sink, opts)
	}, nil
}

func (m *Module) loadChangeCheckpoints(ctx context.Context, spec filament.RunSpec) (map[string]filament.Checkpoint, error) {
	out := make(map[string]filament.Checkpoint, len(spec.Resources))
	for _, resource := range spec.Resources {
		var cp filament.Checkpoint
		var err error
		if key, ok := spec.ResourceCheckpointKey(resource); ok {
			var state filament.ResourceCheckpointState
			state, err = m.ds.LoadResourceCheckpoint(ctx, key)
			cp = state.Checkpoint
		} else {
			cp, err = m.ds.LoadCheckpoint(ctx, spec.Run, resource)
		}
		if err == nil {
			out[resource] = cp
			continue
		}
		if !errors.Is(err, filament.ErrNotFound) {
			return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func isResumableRun(plan filament.IngestionPlan) bool {
	return plan.SourcePolicy.Checkpointing != filament.CheckpointNone && !plan.RequiresCDC
}

// safeCall runs source extraction code, converting a panic into an error so a
// misbehaving connector fails its run rather than crashing the engine's pump.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("source panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}
