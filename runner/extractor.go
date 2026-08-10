package runner

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/galaxy-io/filament"
)

type extractorFunc func(context.Context, filament.RecordSink, filament.ExtractOpts) error

// readPartition is the subset of a run's resources that share one read
// behavior: a change stream for CDC, a resumable read seeded from persisted
// cursors when the policy checkpoints, or a plain full extract. Resources is
// nil when the run covers everything under the wildcard policy.
type readPartition struct {
	policy    filament.SourcePolicy
	resources []string
}

// partitionResources groups the run's resources by read behavior, preserving
// first-seen order. An empty resource list yields one wildcard partition.
func partitionResources(spec filament.RunSpec, plan filament.IngestionPlan) []readPartition {
	if len(spec.Resources) == 0 {
		return []readPartition{{policy: plan.SourcePolicies[""]}}
	}
	type readClass struct {
		mode          filament.ReadMode
		checkpointing filament.CheckpointPolicy
	}
	index := map[readClass]int{}
	var out []readPartition
	for _, resource := range spec.Resources {
		policy := plan.SourcePolicies[resource]
		class := readClass{mode: policy.Mode, checkpointing: policy.Checkpointing}
		i, ok := index[class]
		if !ok {
			i = len(out)
			index[class] = i
			out = append(out, readPartition{policy: policy})
		}
		out[i].resources = append(out[i].resources, resource)
	}
	return out
}

// resolveExtractor builds this run's read side: one sub-extractor per read
// partition, invoked sequentially into the same sink. Seeding resumable
// cursors is the one place a run writes durable state itself; every other
// transition is folded from facts by the tracker.
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
				Resources:   spec.Resources,
				Checkpoints: checkpoints,
				Limit:       opts.Limit,
			})
		}, nil
	}

	partitions := partitionResources(spec, plan)
	extractors := make([]extractorFunc, 0, len(partitions))
	for _, part := range partitions {
		extract, err := resolvePartitionExtractor(ctx, ds, src, spec, part)
		if err != nil {
			return nil, err
		}
		extractors = append(extractors, withPartitionOpts(extract, part))
	}
	if len(extractors) == 1 {
		return extractors[0], nil
	}
	return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
		for _, extract := range extractors {
			if err := extract(ctx, sink, opts); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// withPartitionOpts scopes an extract call to its partition: the partition's
// resources and read mode, serialized when the policy demands order.
func withPartitionOpts(extract extractorFunc, part readPartition) extractorFunc {
	return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
		opts.Resources = part.resources
		opts.Mode = part.policy.Mode
		if part.policy.Ordered {
			opts.Parallelism = 1
		}
		return extract(ctx, sink, opts)
	}
}

// resolvePartitionExtractor picks one partition's read path: a resumable read
// seeded from persisted cursors when the policy checkpoints, or a plain full
// extract.
func resolvePartitionExtractor(ctx context.Context, ds filament.DataStore, src filament.Source, spec filament.RunSpec, part readPartition) (extractorFunc, error) {
	if part.policy.Checkpointing == filament.CheckpointNone {
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			return src.Extract(ctx, sink, opts)
		}, nil
	}
	resumable, ok := src.(filament.Resumable)
	if !ok {
		return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Provider)
	}
	incremental := part.policy.Mode == filament.ModeIncremental
	prev := make(map[string]filament.Checkpoint, len(part.resources))
	for _, resource := range part.resources {
		var cp filament.Checkpoint
		var err error
		if incremental {
			key, valid := spec.ResourceCheckpointKey(resource)
			if !valid {
				return nil, fmt.Errorf("incremental resource %q requires a versioned pipeline route", resource)
			}
			var state filament.ResourceCheckpointState
			state, err = ds.LoadResourceCheckpoint(ctx, key)
			cp = state.Checkpoint
		} else {
			cp, err = ds.LoadCheckpoint(ctx, spec.Run, resource)
		}
		if err == nil {
			prev[resource] = cp
		} else if !errors.Is(err, filament.ErrNotFound) {
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
		resumePlan, err = planner.PlanIncremental(ctx, part.resources, prev, spec.CursorConfigs)
	} else {
		planner, ok := src.(filament.ResumePlanner)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Provider)
		}
		resumePlan, err = planner.PlanResume(ctx, part.resources, prev)
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
			saveErr = ds.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: spec.Run, Checkpoint: cp})
		} else {
			saveErr = ds.SaveCheckpoint(ctx, spec.Run, cp)
		}
		if saveErr != nil {
			return nil, fmt.Errorf("seed checkpoint %q: %w", cp.Resource(), saveErr)
		}
	}
	return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
		return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
	}, nil
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

// isResumableRun reports whether any partition persists cursors worth resuming
// from; a mixed run fails partial so its incremental progress survives.
func isResumableRun(plan filament.IngestionPlan) bool {
	if plan.RequiresCDC {
		return false
	}
	for _, policy := range plan.SourcePolicies {
		if policy.Checkpointing != filament.CheckpointNone {
			return true
		}
	}
	return false
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
