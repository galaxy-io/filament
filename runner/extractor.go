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
func resolveExtractor(ctx context.Context, ds filament.DataStore, src filament.Source, spec filament.RunSpec, plan filament.IngestionPlan, log filament.Logger) (extractorFunc, error) {
	if plan.RequiresCDC {
		changes, ok := src.(filament.ChangeSource)
		if !ok {
			return nil, fmt.Errorf("source %q does not support CDC extraction", spec.Source.Connector)
		}
		checkpoints, err := loadChangeCheckpoints(ctx, ds, spec)
		if err != nil {
			return nil, err
		}
		logLoadedCheckpoints(log, spec, checkpoints)
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			logExtractionStart(log, spec, "cdc", len(checkpoints), len(checkpoints))
			return changes.ExtractChanges(ctx, sink, filament.ChangeExtractOpts{
				Resources:   opts.Resources,
				Checkpoints: checkpoints,
				Limit:       opts.Limit,
				Observe:     opts.Observe,
			})
		}, nil
	}
	incremental, checkpointed := partitionCheckpointing(spec)
	if len(incremental) == 0 && len(checkpointed) == 0 {
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			logExtractionStart(log, spec, "full", 0, 0)
			return src.Extract(ctx, sink, opts)
		}, nil
	}
	resumable, ok := src.(filament.Resumable)
	if !ok {
		return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Connector)
	}
	resumePlan := map[string]filament.Checkpoint{}
	loaded := 0
	if len(incremental) > 0 {
		planner, ok := src.(filament.IncrementalPlanner)
		if !ok {
			return nil, fmt.Errorf("source %q does not support incremental planning", spec.Source.Connector)
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
				noteLoadedCheckpoint(log, spec, resource, "pipeline", state.Checkpoint, &loaded)
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
			return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Connector)
		}
		prev := make(map[string]filament.Checkpoint, len(checkpointed))
		for _, resource := range checkpointed {
			cp, err := ds.LoadCheckpoint(ctx, spec.Run, resource)
			if err == nil {
				prev[resource] = cp
				noteLoadedCheckpoint(log, spec, resource, "run", cp, &loaded)
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
		logExtractionStart(log, spec, "resumable", len(resumePlan), loaded)
		return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
	}, nil
}

// Checkpoint logs intentionally describe cursor shape and scope without logging
// raw cursor values, which may contain database positions, URLs, or opaque tokens.
func logCheckpointLoaded(log filament.Logger, spec filament.RunSpec, resource, scope string, cp filament.Checkpoint) {
	if log == nil {
		return
	}
	mode := filament.SourcePolicyForIngestion(filament.TypeFor(spec.IngestionTypes, resource)).Mode.String()
	log.Info("runner: checkpoint loaded",
		filament.Field{Key: "run", Value: string(spec.Run)},
		filament.Field{Key: "resource", Value: resource},
		filament.Field{Key: "checkpoint_scope", Value: scope},
		filament.Field{Key: "read_mode", Value: mode},
		filament.Field{Key: "checkpoint_kind", Value: filament.CheckpointKind(cp)},
	)
}

func logLoadedCheckpoints(log filament.Logger, spec filament.RunSpec, checkpoints map[string]filament.Checkpoint) {
	for _, resource := range spec.Resources {
		if cp := checkpoints[resource]; cp != nil {
			logCheckpointLoaded(log, spec, resource, checkpointScope(spec, resource), cp)
		}
	}
}

func noteLoadedCheckpoint(
	log filament.Logger,
	spec filament.RunSpec,
	resource, scope string,
	cp filament.Checkpoint,
	loaded *int,
) {
	if cp == nil {
		return
	}
	(*loaded)++
	logCheckpointLoaded(log, spec, resource, scope, cp)
}

func logExtractionStart(log filament.Logger, spec filament.RunSpec, strategy string, checkpointResources, loadedCheckpoints int) {
	if log == nil {
		return
	}
	log.Info("runner: extraction starting",
		filament.Field{Key: "run", Value: string(spec.Run)},
		filament.Field{Key: "strategy", Value: strategy},
		filament.Field{Key: "resources", Value: len(spec.Resources)},
		filament.Field{Key: "checkpoint_resources", Value: checkpointResources},
		filament.Field{Key: "loaded_checkpoints", Value: loadedCheckpoints},
	)
}

func checkpointScope(spec filament.RunSpec, resource string) string {
	if _, ok := spec.ResourceCheckpointKey(resource); ok {
		return "pipeline"
	}
	return "run"
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

// isResumableRun reports whether every resource has progress that is durable
// before Commit and safe to replay. Commit-gated and append progress cannot be
// resumed after a failed attempt without losing or duplicating rows.
func isResumableRun(spec filament.RunSpec, plan filament.IngestionPlan) bool {
	if plan.RequiresCDC {
		return false
	}
	if filament.CheckpointCoverageFor(spec.Resources, spec.IngestionTypes) != filament.CheckpointCoverageAll {
		return false
	}
	incremental, checkpointed := partitionCheckpointing(spec)
	resumeResources := append(append([]string(nil), incremental...), checkpointed...)
	for _, resource := range resumeResources {
		policy, ok := plan.WritePolicies[resource]
		if !ok {
			policy, ok = plan.WritePolicies[""]
		}
		if !ok || policy.Checkpoint == filament.CheckpointAfterCommit ||
			policy.Capability.Mode == filament.WriteAppend {
			return false
		}
	}
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
