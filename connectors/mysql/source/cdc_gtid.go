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

// extractChangesGTID streams row events from the checkpointed GTID set up to the
// watermark captured at run start.
func (s *Source) extractChangesGTID(ctx context.Context, sink filament.RecordSink, opts filament.ChangeExtractOpts, watermark gomysql.GTIDSet) error {
	start, ok, err := startGTID(opts.Checkpoints)
	if err != nil {
		return err
	}
	if !ok {
		start = watermark.Clone() // first run: begin at the current tail
	}

	run := newCDCRun(sink, opts.Resources, opts.Limit)

	if start.Contain(watermark) {
		return run.pushStreamMarksLSN(gtidCursor(start))
	}

	syncer := replication.NewBinlogSyncer(s.binlogConfig())
	defer syncer.Close()
	streamer, err := syncer.StartSyncGTID(start)
	if err != nil {
		return fmt.Errorf("mysql cdc: start gtid sync at %q: %w", start.String(), err)
	}

	committed := start.Clone() // transactions fully delivered — the record cursor
	var inflight string        // GTID of the transaction whose rows are streaming

	fold := func() error {
		if inflight == "" {
			return nil
		}
		if err := committed.Update(inflight); err != nil {
			return fmt.Errorf("mysql cdc: fold gtid %q: %w", inflight, err)
		}
		inflight = ""
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
				return nil // truncated: resume replays from the committed cursor
			}
		}

		if committed.Contain(watermark) {
			return run.pushStreamMarksLSN(gtidCursor(committed))
		}
	}
}

// gtidCursor renders a GTID set as a ModeStream cursor value.
func gtidCursor(set gomysql.GTIDSet) string { return gtidCursorPrefix + set.String() }

// startGTID picks the resume set across the run's resources: the cursor contained
// by all the others (per-resource cursors form a monotone chain, so a minimum
// exists; re-delivering from an older set is safe, skipping is not). Incomparable
// sets mean the checkpoints came from different streams — fail rather than guess.
func startGTID(cps map[string]filament.Checkpoint) (gomysql.GTIDSet, bool, error) {
	var minSet gomysql.GTIDSet
	for _, cp := range cps {
		lsn, _, ok := checkpoint.ParseStream(cp)
		if !ok || !strings.HasPrefix(lsn, gtidCursorPrefix) {
			continue
		}
		set, err := gomysql.ParseMysqlGTIDSet(strings.TrimPrefix(lsn, gtidCursorPrefix))
		if err != nil {
			return nil, false, fmt.Errorf("mysql cdc: parse gtid cursor %q: %w", lsn, err)
		}
		switch {
		case minSet == nil:
			minSet = set
		case minSet.Contain(set):
			minSet = set
		case set.Contain(minSet):
			// keep minSet
		default:
			return nil, false, fmt.Errorf("mysql cdc: incomparable gtid cursors %q and %q (mixed streams?)", minSet.String(), set.String())
		}
	}
	return minSet, minSet != nil, nil
}
