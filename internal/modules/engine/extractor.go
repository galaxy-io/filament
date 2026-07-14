package engine

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	ingestion "github.com/galaxy-io/filament"
)

type extractorFunc func(context.Context, ingestion.RecordSink, ingestion.ExtractOpts) error

func (m *Module) resolveExtractor(ctx context.Context, src ingestion.Source, spec ingestion.RunSpec, plan ingestion.IngestionPlan) (extractorFunc, error) {
	if plan.Type == ingestion.IngestionCDC {
		changes, ok := src.(ingestion.ChangeSource)
		if !ok {
			return nil, fmt.Errorf("source %q does not support CDC extraction", spec.Source.Provider)
		}
		checkpoints, err := m.loadChangeCheckpoints(ctx, spec)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
			return changes.ExtractChanges(ctx, sink, ingestion.ChangeExtractOpts{
				Resources:   opts.Resources,
				Checkpoints: checkpoints,
				Limit:       opts.Limit,
			})
		}, nil
	}
	if isResumableRun(plan) {
		planner, ok := src.(ingestion.ResumePlanner)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Provider)
		}
		resumable, ok := src.(ingestion.Resumable)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Provider)
		}
		prev := make(map[string]ingestion.Checkpoint, len(spec.Resources))
		for _, resource := range spec.Resources {
			cp, err := m.ds.LoadCheckpoint(ctx, spec.Run, resource)
			if err == nil {
				prev[resource] = cp
			} else if err != nil && !errors.Is(err, ingestion.ErrNotFound) {
				return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
			}
		}
		resumePlan, err := planner.PlanResume(ctx, spec.Resources, prev)
		if err != nil {
			return nil, fmt.Errorf("plan resume: %w", err)
		}
		for _, cp := range resumePlan {
			if cp == nil {
				continue
			}
			if err := m.ds.SaveCheckpoint(ctx, spec.Run, cp); err != nil {
				return nil, fmt.Errorf("seed checkpoint %q: %w", cp.Resource(), err)
			}
		}
		return func(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
			return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
		}, nil
	}
	return func(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
		return src.Extract(ctx, sink, opts)
	}, nil
}

func (m *Module) loadChangeCheckpoints(ctx context.Context, spec ingestion.RunSpec) (map[string]ingestion.Checkpoint, error) {
	out := make(map[string]ingestion.Checkpoint, len(spec.Resources))
	for _, resource := range spec.Resources {
		cp, err := m.ds.LoadCheckpoint(ctx, spec.Run, resource)
		if err == nil {
			out[resource] = cp
			continue
		}
		if !errors.Is(err, ingestion.ErrNotFound) {
			return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func isResumableRun(plan ingestion.IngestionPlan) bool {
	return plan.Type == ingestion.IngestionSnapshotUpsert
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
