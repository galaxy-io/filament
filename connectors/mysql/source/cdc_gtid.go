package mysql

// GTID-based CDC streaming — the default whenever the server runs with
// gtid_mode=ON (cdc.go's file:pos path remains the fallback for servers
// without it). A GTID cursor survives replica promotion: the set of executed
// transactions is global to the topology, while binlog file names and offsets
// are local to one server.
//
// GTID granularity is the transaction, not the event, and a transaction's
// GTID event arrives BEFORE its row events. Records therefore carry the
// cursor as of the last COMMITTED transaction (the in-flight GTID folds into
// the running set only at commit): a run that dies mid-transaction resumes by
// replaying that whole transaction, which the idempotent merge absorbs —
// checkpointing the in-flight GTID early would instead skip its undelivered
// tail on resume.

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	gomysql "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

// gtidCursorPrefix discriminates a GTID stream cursor from a file:pos one in
// the shared ModeStream checkpoint payload.
const gtidCursorPrefix = "gtid:"

// gtidMode reports whether the server runs with gtid_mode=ON.
func (s *Source) gtidMode(ctx context.Context) (bool, error) {
	var mode string
	if err := s.db.QueryRowContext(ctx, "SELECT @@gtid_mode").Scan(&mode); err != nil {
		return false, fmt.Errorf("mysql cdc: read gtid_mode: %w", err)
	}
	return strings.EqualFold(mode, "ON"), nil
}

// gtidExecuted reads the server's executed-transaction set — the GTID watermark.
func (s *Source) gtidExecuted(ctx context.Context) (gomysql.GTIDSet, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, "SELECT @@global.gtid_executed").Scan(&raw); err != nil {
		return nil, fmt.Errorf("mysql cdc: read gtid_executed: %w", err)
	}
	set, err := gomysql.ParseMysqlGTIDSet(strings.ReplaceAll(raw, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("mysql cdc: parse gtid_executed %q: %w", raw, err)
	}
	return set, nil
}

// extractChangesGTID bootstraps the resources without a checkpoint, then streams
// row events from the oldest resource floor up to the executed set captured
// after the bootstrap. A resource's floor is a GTID set: the transactions
// already delivered to it, by its snapshot or by an earlier cycle.
func (s *Source) extractChangesGTID(ctx context.Context, run *cdcRun, opts filament.ChangeExtractOpts, bootstrap []string) error {
	floors, watermark, err := s.gtidBootstrap(ctx, run, opts.Checkpoints, bootstrap)
	if err != nil {
		return err
	}
	start, err := oldestGTID(floors)
	if err != nil {
		return err
	}

	if start.Contain(watermark) {
		return run.pushStreamMarks(ctx, s, gtidCursor(start))
	}

	syncer := replication.NewBinlogSyncer(s.binlogConfig())
	defer syncer.Close()
	// The syncer keeps the set it is given and advances it as it reads ahead;
	// hand it a copy so start and the floors it aliases stay fixed.
	streamer, err := syncer.StartSyncGTID(start.Clone())
	if err != nil {
		return fmt.Errorf("mysql cdc: start gtid sync at %q: %w", start.String(), err)
	}

	committed := start.Clone()      // transactions fully delivered — the record cursor
	var inflight string             // GTID of the transaction whose rows are streaming
	var inflightSet gomysql.GTIDSet // the same GTID as a set, for floor checks

	// A row event belongs to the in-flight transaction; it was delivered already
	// when the resource's floor contains that transaction.
	run.skip = func(resource string) bool {
		floor := floors[resource]
		return inflightSet != nil && floor != nil && floor.Contain(inflightSet)
	}

	fold := func() error {
		if inflight == "" {
			return nil
		}
		if err := committed.Update(inflight); err != nil {
			return fmt.Errorf("mysql cdc: fold gtid %q: %w", inflight, err)
		}
		inflight, inflightSet = "", nil
		return nil
	}

	for {
		ev, err := streamer.GetEvent(ctx)
		if err != nil {
			return fmt.Errorf("mysql cdc: read event at %q: %w", committed.String(), err)
		}

		switch e := ev.Event.(type) {
		case *replication.GTIDEvent:
			// Header of the next transaction: the previous one (if any) is fully
			// delivered — fold it before tracking the new in-flight GTID.
			if err := fold(); err != nil {
				return err
			}
			sid, err := uuid.FromBytes(e.SID)
			if err != nil {
				return fmt.Errorf("mysql cdc: gtid sid: %w", err)
			}
			inflight = fmt.Sprintf("%s:%d", sid, e.GNO)
			if inflightSet, err = gomysql.ParseMysqlGTIDSet(inflight); err != nil {
				return fmt.Errorf("mysql cdc: parse gtid %q: %w", inflight, err)
			}
		case *replication.XIDEvent:
			if err := fold(); err != nil { // transaction commit
				return err
			}
		case *replication.QueryEvent:
			// reject events for transactions
			switch q := strings.ToUpper(strings.TrimSpace(string(e.Query))); {
			case q == "BEGIN", q == "COMMIT", q == "ROLLBACK", strings.HasPrefix(q, "SAVEPOINT "), strings.HasPrefix(q, "ROLLBACK TO "):
				// not a commit boundary
			default:
				if err := fold(); err != nil {
					return err
				}
				run.clearSchema()
			}
		case *replication.RowsEvent:
			// Records carry the committed-set cursor: resuming from it replays the
			// current (uncommitted-at-cursor-time) transaction in full.
			limited, err := run.pushRowsEvent(ctx, s, ev.Header.EventType, e, gtidCursor(committed))
			if err != nil {
				return err
			}
			if limited {
				// Truncated: mark every resource at the committed cursor so a
				// resource that saw no rows this cycle (a just-bootstrapped one in
				// particular) resumes from here rather than bootstrapping again.
				return run.pushStreamMarks(ctx, s, gtidCursor(committed))
			}
		}

		if committed.Contain(watermark) {
			return run.pushStreamMarks(ctx, s, gtidCursor(committed))
		}
	}
}

// gtidBootstrap decodes the checkpointed floors, snapshots the resources without
// one, and captures the cycle's watermark afterwards. gtid_executed admits a
// transaction only once its engine commit is done, so every transaction in a
// bootstrap floor is visible to the snapshot opened after it; everything else is
// replayed by the stream.
func (s *Source) gtidBootstrap(ctx context.Context, run *cdcRun, cps map[string]filament.Checkpoint, bootstrap []string) (map[string]gomysql.GTIDSet, gomysql.GTIDSet, error) {
	floors, err := gtidFloors(cps)
	if err != nil {
		return nil, nil, err
	}
	if len(bootstrap) > 0 {
		floor, err := s.gtidExecuted(ctx)
		if err != nil {
			return nil, nil, err
		}
		if err := s.snapshotBootstrap(ctx, run, bootstrap); err != nil {
			return nil, nil, err
		}
		for _, resource := range bootstrap {
			floors[resource] = floor
		}
	}
	watermark, err := s.gtidExecuted(ctx)
	if err != nil {
		return nil, nil, err
	}
	return floors, watermark, nil
}

// gtidCursor renders a GTID set as a ModeStream cursor value.
func gtidCursor(set gomysql.GTIDSet) string { return gtidCursorPrefix + set.String() }

// gtidFloors decodes each resource's checkpointed GTID cursor: the set of
// transactions already delivered to it.
func gtidFloors(cps map[string]filament.Checkpoint) (map[string]gomysql.GTIDSet, error) {
	floors := make(map[string]gomysql.GTIDSet, len(cps))
	for resource, cp := range cps {
		if cp == nil {
			continue
		}
		cursor, _, ok := checkpoint.ParseStream(cp)
		if !ok || !strings.HasPrefix(cursor, gtidCursorPrefix) {
			return nil, fmt.Errorf("mysql cdc: checkpoint for %q is not a gtid cursor", resource)
		}
		set, err := gomysql.ParseMysqlGTIDSet(strings.TrimPrefix(cursor, gtidCursorPrefix))
		if err != nil {
			return nil, fmt.Errorf("mysql cdc: parse gtid cursor %q: %w", cursor, err)
		}
		floors[resource] = set
	}
	return floors, nil
}

// oldestGTID picks the stream start across the run's resources: the floor
// contained by all the others (per-resource floors form a monotone chain, so a
// minimum exists; re-delivering from an older set is safe, skipping is not).
// Incomparable sets mean the checkpoints came from different streams — fail
// rather than guess.
func oldestGTID(floors map[string]gomysql.GTIDSet) (gomysql.GTIDSet, error) {
	var minSet gomysql.GTIDSet
	for _, set := range floors {
		switch {
		case minSet == nil:
			minSet = set
		case minSet.Contain(set):
			minSet = set
		case set.Contain(minSet):
			// keep minSet
		default:
			return nil, fmt.Errorf("mysql cdc: incomparable gtid cursors %q and %q (mixed streams?)", minSet.String(), set.String())
		}
	}
	return minSet, nil
}
