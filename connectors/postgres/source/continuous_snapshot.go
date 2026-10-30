package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pglogrepl"

	"github.com/galaxy-io/filament"
)

// continuousSnapshot keeps the baseline and its WAL handoff in one epoch. No
// snapshot checkpoint is returned until every selected table has been read.
type continuousSnapshot struct {
	source   *Source
	snapshot *snapshot
	shards   []shard
	target   pglogrepl.LSN
}

func (s *Source) planContinuousSnapshot(ctx context.Context, opts filament.StreamOpenOpts) (*continuousSnapshot, error) {
	if s.snapshotMode != filament.SnapshotInitial || len(opts.CommittedPositions) != 0 {
		return nil, nil
	}
	shards, err := s.planShards(ctx, opts.Resources, 1)
	if err != nil {
		return nil, err
	}
	return &continuousSnapshot{source: s, shards: shards}, nil
}

func (b *continuousSnapshot) open(ctx context.Context, slot replicationSlotState) error {
	b.target = slot.start
	snapshotID := slot.snapshotID
	if snapshotID == "" {
		// An interrupted bootstrap has a retained slot but its exported snapshot
		// is gone. Export a new consistent snapshot without replacing that slot.
		// Replay the retained WAL through this newer point before certifying it,
		// including deletes of rows a previous attempt may already have written.
		exporter, err := b.source.replicationConn(ctx)
		if err != nil {
			return err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = exporter.Close(cleanup)
		}()
		created, err := pglogrepl.CreateReplicationSlot(ctx, exporter, fmt.Sprintf("filament_snapshot_%d", exporter.PID()), "pgoutput", pglogrepl.CreateReplicationSlotOptions{Temporary: true, SnapshotAction: "EXPORT_SNAPSHOT"})
		if err != nil {
			return fmt.Errorf("postgres continuous: export retry snapshot: %w", err)
		}
		b.target, err = pglogrepl.ParseLSN(created.ConsistentPoint)
		if err != nil {
			return err
		}
		snapshotID = created.SnapshotName
		// Import before closing the exporter. The importing transaction retains
		// the snapshot after the temporary slot and its connection are released.
		b.snapshot, err = b.source.importSnapshot(ctx, snapshotID)
		return err
	}
	var err error
	b.snapshot, err = b.source.importSnapshot(ctx, snapshotID)
	return err
}

func (b *continuousSnapshot) close(ctx context.Context) {
	if b != nil && b.snapshot != nil {
		b.snapshot.close(ctx)
		b.snapshot = nil
	}
}

func (s *walSession) readSnapshot(ctx context.Context, out filament.StreamRecordSink) error {
	b := s.bootstrap
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		b.close(cleanup)
		s.bootstrap = nil
	}()
	control := filament.Control{Kind: filament.TxnBegin, Domain: s.domain, Position: walPosition(s.start), TxnID: "snapshot"}
	if err := out.Control(ctx, control); err != nil {
		return err
	}
	// Planning precedes importing the snapshot so a one-connection pool works.
	// Refresh heap bounds through the snapshot transaction: rows committed
	// between planning and snapshot creation may occupy additional pages.
	for i := range b.shards {
		if err := b.snapshot.tx.QueryRow(ctx, `SELECT (pg_relation_size($1::regclass) / current_setting('block_size')::bigint)::int`, b.shards[i].qualified).Scan(&b.shards[i].hiBlock); err != nil {
			return err
		}
	}
	if err := b.source.extractInitialSnapshot(ctx, s.decoder, b.snapshot.tx, b.shards); err != nil {
		return err
	}
	control.Kind = filament.TxnEnd
	return out.Control(ctx, control)
}
