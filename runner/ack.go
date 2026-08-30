package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

const durableChangeAckWait = 5 * time.Second

// acknowledgeDurableChanges releases source-side stream retention once the
// tracker has promoted every final cursor into the cross-run checkpoint store.
// Failure is deliberately non-terminal: the sink is already committed, and the
// next CDC cycle retries the acknowledgement. Detached from run cancellation so
// shutdown cannot skip it, bounded so a lagging tracker cannot hang the worker.
func acknowledgeDurableChanges(
	ctx context.Context,
	deps Deps,
	spec filament.RunSpec,
	src filament.Source,
	targets map[string]filament.Checkpoint,
) {
	ack, ok := src.(filament.ChangeAcknowledger)
	if !ok || deps.DataStore == nil || len(targets) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), durableChangeAckWait)
	defer cancel()
	durable, err := waitForDurableStreamCheckpoints(ctx, deps.DataStore, spec, targets)
	if err == nil {
		err = ack.AcknowledgeChanges(ctx, durable)
	}
	if err != nil && deps.Log != nil {
		deps.Log.Warn("runner: acknowledge durable change checkpoint",
			filament.Field{Key: "run", Value: string(spec.Run)},
			filament.Field{Key: "error", Value: err.Error()})
	}
}

func waitForDurableStreamCheckpoints(
	ctx context.Context,
	ds filament.DataStore,
	spec filament.RunSpec,
	targets map[string]filament.Checkpoint,
) (map[string]filament.Checkpoint, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		durable := make(map[string]filament.Checkpoint, len(targets))
		ready := true
		for resource, target := range targets {
			key, ok := spec.ResourceCheckpointKey(resource)
			if !ok {
				return nil, fmt.Errorf("CDC resource %q has no durable checkpoint route", resource)
			}
			state, err := ds.LoadResourceCheckpoint(ctx, key)
			if err != nil {
				if errors.Is(err, filament.ErrNotFound) {
					ready = false
					break
				}
				return nil, err
			}
			if !streamCheckpointReached(state.Checkpoint, target) {
				ready = false
				break
			}
			durable[resource] = state.Checkpoint
		}
		if ready {
			return durable, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func streamCheckpointReached(durable, target filament.Checkpoint) bool {
	durableLSN, durableSeq, durableOK := checkpoint.ParseStream(durable)
	targetLSN, targetSeq, targetOK := checkpoint.ParseStream(target)
	if !durableOK || !targetOK || durableSeq < targetSeq {
		return false
	}
	return durableSeq > targetSeq || durableLSN == targetLSN
}
