package runner

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/galaxy-io/filament"
)

type extractorFunc func(context.Context, filament.RecordSink, filament.ExtractOpts) error

// resolveExtractor picks how this run reads its source: a change stream for
// CDC, a resumable read seeded from persisted cursors when any resource's
// type checkpoints, or a plain full extract. Read behavior is per resource —
// incremental resources plan from cross-run cursors, checkpointing full
// resources resume from run-scoped checkpoints, and checkpoint-free resources
// carry no plan entry, which sources read whole. Seeding those cursors is the
// one place a run writes durable state itself; every other transition is
// folded from facts by the tracker.
func resolveExtractor(ctx context.Context, ds filament.DataStore, src filament.Source, spec filament.RunSpec, plan filament.IngestionPlan) (extractorFunc, error) {
	if plan.RequiresCDC {
		changes, ok := src.(filament.ChangeSource)
		if !ok {
			return nil, fmt.Errorf("source %q does not support CDC extraction", spec.Source.Provider)
		}
		checkpoints, err := loadChangeCheckpoints(ctx, ds, spec)
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
	incremental, checkpointed := partitionCheckpointing(spec)
	if len(incremental) == 0 && len(checkpointed) == 0 {
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			return src.Extract(ctx, sink, opts)
		}, nil
	}
	resumable, ok := src.(filament.Resumable)
	if !ok {
		return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Provider)
	}
	resumePlan := map[string]filament.Checkpoint{}
	if len(incremental) > 0 {
		planner, ok := src.(filament.IncrementalPlanner)
		if !ok {
			return nil, fmt.Errorf("source %q does not support incremental planning", spec.Source.Provider)
		}
		prev := make(map[string]filament.Checkpoint, len(incremental))
		for _, resource := range incremental {
			key, valid := spec.ResourceCheckpointKey(resource)
			if !valid {
				return nil, fmt.Errorf("incremental resource %q requires a versioned pipeline route", resource)
			}
			state, err := ds.LoadResourceCheckpoint(ctx, key)
			if err == nil {
				prev[resource] = state.Checkpoint
			} else if !errors.Is(err, filament.ErrNotFound) {
				return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
			}
		}
		planned, err := planner.PlanIncremental(ctx, incremental, prev, spec.CursorConfigs)
		if err != nil {
			return nil, fmt.Errorf("plan incremental: %w", err)
		}
		for resource, cp := range planned {
			if cp == nil {
				continue
			}
			key, _ := spec.ResourceCheckpointKey(resource)
			if err := ds.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: spec.Run, Checkpoint: cp}); err != nil {
				return nil, fmt.Errorf("seed checkpoint %q: %w", resource, err)
			}
			resumePlan[resource] = cp
		}
	}
	if len(checkpointed) > 0 {
		planner, ok := src.(filament.ResumePlanner)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Provider)
		}
		prev := make(map[string]filament.Checkpoint, len(checkpointed))
		for _, resource := range checkpointed {
			cp, err := ds.LoadCheckpoint(ctx, spec.Run, resource)
			if err == nil {
				prev[resource] = cp
			} else if !errors.Is(err, filament.ErrNotFound) {
				return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
			}
		}
		planned, err := planner.PlanResume(ctx, checkpointed, prev)
		if err != nil {
			return nil, fmt.Errorf("plan resume: %w", err)
		}
		for resource, cp := range planned {
			if cp == nil {
				continue
			}
			if err := ds.SaveCheckpoint(ctx, spec.Run, cp); err != nil {
				return nil, fmt.Errorf("seed checkpoint %q: %w", resource, err)
			}
			resumePlan[resource] = cp
		}
	}
	return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
		return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
	}, nil
}

// partitionCheckpointing splits the run's resources by their type's read-side
// checkpoint behavior: incremental resources cursor across runs, checkpointed
// resources resume within a run, and everything else (checkpoint-free full
// reads) is omitted.
func partitionCheckpointing(spec filament.RunSpec) (incremental, checkpointed []string) {
	for _, resource := range spec.Resources {
		policy := filament.SourcePolicyForIngestion(filament.TypeFor(spec.IngestionTypes, resource))
		switch {
		case policy.Mode == filament.ModeIncremental:
			incremental = append(incremental, resource)
		case policy.Checkpointing != filament.CheckpointNone:
			checkpointed = append(checkpointed, resource)
		}
	}
	return incremental, checkpointed
}

func loadChangeCheckpoints(ctx context.Context, ds filament.DataStore, spec filament.RunSpec) (map[string]filament.Checkpoint, error) {
	out := make(map[string]filament.Checkpoint, len(spec.Resources))
	for _, resource := range spec.Resources {
		var cp filament.Checkpoint
		var err error
		if key, ok := spec.ResourceCheckpointKey(resource); ok {
			var state filament.ResourceCheckpointState
			state, err = ds.LoadResourceCheckpoint(ctx, key)
			cp = state.Checkpoint
		} else {
			cp, err = ds.LoadCheckpoint(ctx, spec.Run, resource)
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

// isResumableRun reports whether a failure can leave resumable progress: some
// resource checkpoints and the run is not a CDC catch-up cycle.
func isResumableRun(spec filament.RunSpec, plan filament.IngestionPlan) bool {
	if plan.RequiresCDC {
		return false
	}
	incremental, checkpointed := partitionCheckpointing(spec)
	return len(incremental)+len(checkpointed) > 0
}

// safeCall runs source extraction code, converting a panic into an error so a
// misbehaving connector fails its run rather than crashing the caller's pump.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("source panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}
