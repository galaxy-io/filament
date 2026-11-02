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
// A stream's first baseline covers every table from the slot's own snapshot.
// A later one covers only the pending members, exported at a fresh consistent
// point that also floors their WAL, so the rows the baseline already holds
// are not replayed.
type continuousSnapshot struct {
	source   *Source
	snapshot *snapshot
	shards   []shard
	target   pglogrepl.LSN
	// members are the pending tables this baseline admits; empty on a stream's
	// first baseline, whose tables are already members.
	members []string
}

func (s *Source) planContinuousSnapshot(ctx context.Context, opts filament.StreamOpenOpts) (*continuousSnapshot, error) {
	if s.snapshotMode != filament.SnapshotInitial {
		return nil, nil
	}
	tables := opts.Resources
	var members []string
	if len(opts.CommittedPositions) != 0 {
		members = pendingMembers(opts)
		if len(members) == 0 {
			return nil, nil
		}
		tables = members
	}
	shards, err := s.planShards(ctx, tables, 1)
	if err != nil {
		return nil, err
	}
	return &continuousSnapshot{source: s, shards: shards, members: members}, nil
}

// pendingMembers lists the selected resources that have no baseline yet.
func pendingMembers(opts filament.StreamOpenOpts) []string {
	var pending []string
	for _, resource := range opts.Resources {
		if status, ok := opts.Membership[resource]; ok && status == filament.ReplicationStreamResourcePending {
			pending = append(pending, resource)
		}
	}
	return pending
}

func (b *continuousSnapshot) open(ctx context.Context, slot replicationSlotState) error {
	if b == nil {
		return nil
	}
	b.target = slot.start
	snapshotID := slot.snapshotID
	if len(b.members) > 0 {
		snapshotID = ""
	}
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
	if err := out.Control(ctx, control); err != nil {
		return err
	}
	for _, member := range b.members {
		if err := out.Control(ctx, filament.Control{Kind: filament.MemberActivated, Domain: s.domain, Position: walPosition(b.target), Resource: member, MembershipRevision: s.membershipRevision}); err != nil {
			return err
		}
	}
	return nil
}
