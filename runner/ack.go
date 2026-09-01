package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

// durableChangeAckWait bounds one acknowledgement. It must outlast both a
// lagging tracker promotion and the source's own wait for a peer to release
// the stream.
const durableChangeAckWait = 30 * time.Second

// acknowledgeDurableChanges releases source-side stream retention up to the
// route floor: the oldest durable cursor of every resource under the run's
// checkpoint route, not only this run's. A run covering a subset of the
// route's resources must never advance past the ones it left out.
//
// Called at run start, when everything in the store is already durable, and
// at run end once the tracker has promoted this run's final cursors (targets).
// An acknowledgement skipped at the end of one run is therefore repeated at
// the start of the next. Failure is non-terminal: the sink is already
// committed. Detached from run cancellation so shutdown cannot skip it,
// bounded so a lagging tracker cannot hang the worker.
func acknowledgeDurableChanges(
	ctx context.Context,
	deps Deps,
	spec filament.RunSpec,
	src filament.Source,
	targets map[string]filament.Checkpoint,
) {
	ack, ok := src.(filament.ChangeAcknowledger)
	if !ok || deps.DataStore == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), durableChangeAckWait)
	defer cancel()
	err := waitForDurableStreamCheckpoints(ctx, deps.DataStore, spec, targets)
	var floor map[string]filament.Checkpoint
	if err == nil {
		floor, err = routeStreamCheckpoints(ctx, deps.DataStore, spec)
	}
	if err == nil && len(floor) > 0 {
		err = ack.AcknowledgeChanges(ctx, floor)
	}
	if err != nil && deps.Log != nil {
		deps.Log.Warn("durable change checkpoint not acknowledged",
			filament.Field{Key: "event.name", Value: "runner.checkpoint.acknowledge_failed"},
			filament.Field{Key: "error", Value: err.Error()})
	}
}

// waitForDurableStreamCheckpoints blocks until the store holds a cursor at or
// beyond each target, i.e. until the tracker has promoted this run's progress.
func waitForDurableStreamCheckpoints(
	ctx context.Context,
	ds filament.DataStore,
	spec filament.RunSpec,
	targets map[string]filament.Checkpoint,
) error {
	if len(targets) == 0 {
		return nil
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		ready := true
		for resource, target := range targets {
			key, ok := spec.ResourceCheckpointKey(resource)
			if !ok {
				return fmt.Errorf("CDC resource %q has no durable checkpoint route", resource)
			}
			state, err := ds.LoadResourceCheckpoint(ctx, key)
			if errors.Is(err, filament.ErrNotFound) || (err == nil && !streamCheckpointReached(state.Checkpoint, target)) {
				ready = false
				break
			}
			if err != nil {
				return err
			}
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// routeStreamCheckpoints returns every durable stream cursor under the run's
// checkpoint route, keyed by resource. Cursors of other shapes (incremental
// columns) share the route but not the stream and are left out.
func routeStreamCheckpoints(ctx context.Context, ds filament.DataStore, spec filament.RunSpec) (map[string]filament.Checkpoint, error) {
	if spec.PipelineID == "" || spec.PipelineVersionID == "" || spec.CheckpointRoute == "" {
		return nil, nil
	}
	states, err := ds.ListResourceCheckpoints(ctx, filament.ResourceCheckpointRoute{
		PipelineID: spec.PipelineID, PipelineVersionID: spec.PipelineVersionID, Route: spec.CheckpointRoute,
	})
	if err != nil {
		return nil, err
	}
	floor := make(map[string]filament.Checkpoint, len(states))
	for _, state := range states {
		if _, _, ok := checkpoint.ParseStream(state.Checkpoint); ok {
			floor[state.Key.Resource] = state.Checkpoint
		}
	}
	return floor, nil
}

func streamCheckpointReached(durable, target filament.Checkpoint) bool {
	durableLSN, durableSeq, durableOK := checkpoint.ParseStream(durable)
	targetLSN, targetSeq, targetOK := checkpoint.ParseStream(target)
	if !durableOK || !targetOK || durableSeq < targetSeq {
		return false
	}
	return durableSeq > targetSeq || durableLSN == targetLSN
}
