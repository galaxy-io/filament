package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
)

const replicationCleanupWait = 30 * time.Second

// bindReplicationStream loads the stream admitted with the run and lets the
// connector add its runtime-only consumer configuration.
func bindReplicationStream(ctx context.Context, ds filament.DataStore, src filament.Source, spec *filament.RunSpec) error {
	if spec.ReplicationStreamID == "" {
		return nil
	}
	store, ok := ds.(filament.ReplicationStreamStore)
	if !ok {
		return fmt.Errorf("datastore does not support replication stream %q", spec.ReplicationStreamID)
	}
	planner, ok := src.(filament.ReplicationStreamPlanner)
	if !ok {
		return fmt.Errorf("source %q cannot bind replication stream %q", spec.Source.Connector, spec.ReplicationStreamID)
	}
	stream, err := store.LoadReplicationStream(ctx, spec.ReplicationStreamID)
	if err != nil {
		return fmt.Errorf("load replication stream %q: %w", spec.ReplicationStreamID, err)
	}
	if stream.Status != filament.ReplicationStreamActive || stream.Generation != spec.ReplicationStreamGeneration {
		return fmt.Errorf("replication stream %q generation %d is no longer active", spec.ReplicationStreamID, spec.ReplicationStreamGeneration)
	}
	spec.Source.Config, err = planner.BindReplicationStream(spec.Source.Config, stream)
	if err != nil {
		return fmt.Errorf("bind replication stream %q: %w", spec.ReplicationStreamID, err)
	}
	return nil
}

// cleanupRetiredReplicationStreams releases predecessor consumers only after
// the successor has committed. Consumers on a different connection are left
// for explicit operator cleanup because this source is not configured for that
// system anymore.
func cleanupRetiredReplicationStreams(ctx context.Context, deps Deps, spec filament.RunSpec, src filament.Source) {
	if spec.ReplicationStreamID == "" || deps.DataStore == nil {
		return
	}
	store, ok := deps.DataStore.(filament.ReplicationStreamStore)
	if !ok {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replicationCleanupWait)
	defer cancel()
	retiredStreams, err := store.ListRetiredReplicationStreams(cleanupCtx, spec.PipelineID, spec.CheckpointRoute)
	if err != nil {
		if deps.Log != nil {
			deps.Log.Warn("retired replication consumers could not be listed",
				filament.Field{Key: "event.name", Value: "runner.replication.cleanup_list_failed"},
				filament.Field{Key: "error", Value: err.Error()})
		}
		return
	}
	if len(retiredStreams) == 0 {
		return
	}
	cleaner, ok := src.(filament.ReplicationStreamCleaner)
	if !ok {
		if deps.Log != nil {
			deps.Log.Warn("source cannot clean retired replication consumers",
				filament.Field{Key: "event.name", Value: "runner.replication.cleanup_unsupported"})
		}
		return
	}
	for _, retired := range retiredStreams {
		if retired.SourceConnectionID != spec.SourceConnectionID {
			if deps.Log != nil {
				deps.Log.Warn("retired replication consumer requires cleanup on its original connection",
					filament.Field{Key: "event.name", Value: "runner.replication.cleanup_deferred"},
					filament.Field{Key: "replication_stream_id", Value: retired.ID},
					filament.Field{Key: "consumer_name", Value: retired.ConsumerName})
			}
			continue
		}
		if err := cleaner.CleanupReplicationStream(cleanupCtx, retired); err != nil {
			if deps.Log != nil {
				deps.Log.Warn("retired replication consumer cleanup failed",
					filament.Field{Key: "event.name", Value: "runner.replication.cleanup_failed"},
					filament.Field{Key: "replication_stream_id", Value: retired.ID},
					filament.Field{Key: "consumer_name", Value: retired.ConsumerName},
					filament.Field{Key: "error", Value: err.Error()})
			}
			continue
		}
		if err := store.MarkReplicationStreamCleaned(cleanupCtx, retired.ID); err != nil && deps.Log != nil {
			deps.Log.Warn("retired replication consumer cleanup could not be recorded",
				filament.Field{Key: "event.name", Value: "runner.replication.cleanup_mark_failed"},
				filament.Field{Key: "replication_stream_id", Value: retired.ID},
				filament.Field{Key: "error", Value: err.Error()})
		}
	}
}
