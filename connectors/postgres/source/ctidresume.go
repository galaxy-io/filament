package postgres

// ctid + xmin physical read mode.
// The general low-correlation tier: read physical heap block ranges (fast sequential I/O)
// and resume per block-range shard via Done, exactly like bitmap — but with three read-only
// guards that make the physical cursor catch arbitrary churn, including PK moves, across a
// resume boundary:
//
//  1. Within-run stability: when parallelism > 1 all shards read under one exported snapshot;
//     in the serial path each shard opens its own transaction. MVCC keeps an updated row's old
//     version visible at its original ctid, so a block range is stable within a transaction.
//  2. Rewrite guard: the run-start pg_relation_filenode is stamped; a changed filenode on
//     resume (VACUUM FULL / CLUSTER / rewriting ALTER / TRUNCATE) invalidates the cursor and
//     the table is re-read from scratch.
//  3. Horizon-compare reconciliation: the run-start snapshot xmin H1 is stamped; before the
//     run completes, one filtered pass re-reads every completed block range delivering rows
//     whose xmin >= H1 (numeric xid compare via age(), avoiding pg_visible_in_snapshot). That
//     re-delivers anything that churned since run start; the idempotent sink absorbs the
//     over-delivery. A freeze guard rescans unfiltered if relfrozenxid advanced past H1.

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

// ctid checkpoint Meta keys (run-start stamps).
const (
	metaFilenode     = "filenode"
	metaRelfrozenxid = "relfrozenxid"
	metaXminHorizon  = "xmin"     // H1: pg_snapshot_xmin at run start
	metaRelpages     = "relpages" // heap size at run start (diagnostic / tail reference)
)

// planCtid lays out a fresh ctid checkpoint: heap block-range shards (Lo/Hi are block
// numbers) plus the run-start stamps used by the rewrite guard and the reconciliation pass.
func (s *Source) planCtid(ctx context.Context, table string, pks []pkColumn) (checkpoint.KeysetCheckpoint, error) {
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	ks := checkpoint.KeysetCheckpoint{Mode: checkpoint.ModeCtid, Cols: pkNames(pks), Types: pkTypes(pks)}

	pages, err := s.lookupPages(ctx, qualified)
	if err != nil {
		return ks, fmt.Errorf("ctid pages %q: %w", table, err)
	}
	k := s.keyShardCount(ctx, qualified)
	ks.Shards = blockRangeShards(pages, k)

	meta, err := s.stampCtid(ctx, qualified)
	if err != nil {
		return ks, fmt.Errorf("ctid stamps %q: %w", table, err)
	}
	meta[metaRelpages] = strconv.Itoa(pages)
	ks.Meta = meta
	return ks, nil
}

// blockRangeShards carves [0, pages) into k contiguous block ranges, the last ending
// exactly at pages. An empty table is one empty shard that completes immediately.
func blockRangeShards(pages, k int) []checkpoint.KeysetShard {
	if k < 1 {
		k = 1
	}
	shards := make([]checkpoint.KeysetShard, k)
	for i := range k {
		lo := i * pages / k
		hi := (i + 1) * pages / k
		if i == k-1 {
			hi = pages
		}
		shards[i] = checkpoint.KeysetShard{Lo: []string{strconv.Itoa(lo)}, Hi: []string{strconv.Itoa(hi)}}
	}
	return shards
}

// stampCtid captures the run-start filenode, relfrozenxid, and snapshot xmin horizon (H1).
func (s *Source) stampCtid(ctx context.Context, qualified string) (map[string]string, error) {
	const q = `
SELECT pg_relation_filenode($1::regclass)::text,
       (SELECT relfrozenxid::text FROM pg_class WHERE oid = $1::regclass),
       pg_snapshot_xmin(pg_current_snapshot())::text`
	var filenode, relfrozenxid, xmin string
	if err := s.pool.QueryRow(ctx, q, qualified).Scan(&filenode, &relfrozenxid, &xmin); err != nil {
		return nil, err
	}
	return map[string]string{
		metaFilenode:     filenode,
		metaRelfrozenxid: relfrozenxid,
		metaXminHorizon:  xmin,
	}, nil
}

// ctidGuardIntact is the resume rewrite guard: for a ctid checkpoint it reports whether the
// table's filenode still matches the run-start stamp. A mismatch (VACUUM FULL / CLUSTER /
// rewriting ALTER / TRUNCATE) means block ranges no longer map to the same rows, so the
// cursor is invalid and the caller re-plans. Non-ctid checkpoints are always intact.
func (s *Source) ctidGuardIntact(ctx context.Context, table string, ks checkpoint.KeysetCheckpoint) (bool, error) {
	if ks.Mode != checkpoint.ModeCtid {
		return true, nil
	}
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	now, err := s.currentFilenode(ctx, qualified)
	if err != nil {
		return false, fmt.Errorf("ctid rewrite guard %q: %w", table, err)
	}
	return now == ks.Meta[metaFilenode], nil
}

// currentFilenode reads the table's filenode now, for the resume rewrite guard.
func (s *Source) currentFilenode(ctx context.Context, qualified string) (string, error) {
	var fn string
	err := s.pool.QueryRow(ctx, "SELECT pg_relation_filenode($1::regclass)::text", qualified).Scan(&fn)
	return fn, err
}

// ctidJobs builds the resumable ctid read for one table: read every not-Done block range
// (Coarse, ack-counted to completion), and on resume also (a) reconcile each completed range
// by re-delivering rows that churned since the run-start horizon, and (b) tail-scan any heap
// blocks appended past the run-start size (all such rows are post-horizon). Reconcile/tail
// rows are plain (no Part/Coarse) — pure idempotent re-deliveries that touch no cursor.
func (s *Source) ctidJobs(ctx context.Context, sink arrowbatch.Inlet, table, qualified string, ks checkpoint.KeysetCheckpoint, dec *rowDecoder, limit int) []func(context.Context, querier) error {
	shards := s.ctidShardsFrom(ctx, table, qualified, ks, dec)
	horizon := ks.Meta[metaXminHorizon]
	unfiltered := s.freezeAdvanced(ctx, qualified, horizon)

	var jobs []func(context.Context, querier) error
	for i, sh := range shards {
		if ks.Shards[i].Done {
			jobs = append(jobs, func(ctx context.Context, q querier) error {
				return s.reconcileCtidBlocks(ctx, sink, q, sh, horizon, unfiltered)
			})
			continue
		}
		jobs = append(jobs, func(ctx context.Context, q querier) error {
			return s.extractCtidShard(ctx, sink, q, sh, i, limit)
		})
	}

	// Tail: rows appended into blocks beyond the run-start heap never existed at run start,
	// so they are all post-horizon — deliver the whole tail range unfiltered.
	if stamped := atoiOr(toSlice(ks.Meta[metaRelpages]), 0); len(shards) > 0 {
		if cur, err := s.lookupPages(ctx, qualified); err == nil && cur > stamped {
			tail := shards[0]
			tail.loBlock, tail.hiBlock = stamped, cur
			jobs = append(jobs, func(ctx context.Context, q querier) error {
				return s.reconcileCtidBlocks(ctx, sink, q, tail, horizon, true)
			})
		}
	}
	return jobs
}

func toSlice(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// ctidShardsFrom reconstructs block-range shards from a decoded ctid checkpoint.
func (s *Source) ctidShardsFrom(ctx context.Context, table, qualified string, ks checkpoint.KeysetCheckpoint, dec *rowDecoder) []shard {
	window, err := s.windowBlocks(ctx, qualified)
	if err != nil || window < 1 {
		window = 1
	}
	out := make([]shard, len(ks.Shards))
	for i, sh := range ks.Shards {
		out[i] = shard{
			table:        table,
			qualified:    qualified,
			dec:          dec,
			loBlock:      atoiOr(sh.Lo, 0),
			hiBlock:      atoiOr(sh.Hi, 0),
			windowBlocks: window,
		}
	}
	return out
}

// freezeAdvanced reports whether the table's relfrozenxid has advanced past the run-start
// horizon H1. If so, rows committed in [H1, relfrozenxid) may have been frozen — their xmin
// now reads as ancient, so the horizon filter would miss them and the reconcile must rescan
// unfiltered. Errors default to unfiltered (over-deliver is safe; a miss is not).
func (s *Source) freezeAdvanced(ctx context.Context, qualified, horizon string) bool {
	if horizon == "" {
		return false
	}
	const q = `SELECT age($1::xid) > age(relfrozenxid) FROM pg_class WHERE oid = $2::regclass`
	var adv bool
	if err := s.pool.QueryRow(ctx, q, horizon, qualified).Scan(&adv); err != nil {
		return true
	}
	return adv
}

// reconcileCtidBlocks re-delivers rows in a block range that churned since the run-start
// horizon: numeric xid compare age(xmin) <= age(H1) (avoids pg_visible_in_snapshot, which is
// unsafe with subtransaction xmins). unfiltered re-delivers the whole range (freeze guard, or
// the append tail). Rows are plain — idempotent re-deliveries that advance no cursor.
func (s *Source) reconcileCtidBlocks(ctx context.Context, sink arrowbatch.Inlet, q querier, sh shard, horizon string, unfiltered bool) error {
	filter := ""
	var extra []any
	if !unfiltered && horizon != "" {
		filter = " AND age(t.xmin) <= age($3::xid)"
		extra = []any{horizon}
	}
	w, err := sink.Builder(sh.table, 0, sh.dec.schema)
	if err != nil {
		return err
	}
	if _, err := s.readBlocks(ctx, w, q, sh, filter, extra, rowmodel.Meta{}, 0); err != nil {
		return fmt.Errorf("reconcile %q: %w", sh.table, err)
	}
	return nil
}

func atoiOr(s []string, def int) int {
	if len(s) == 0 {
		return def
	}
	n, err := strconv.Atoi(s[0])
	if err != nil {
		return def
	}
	return n
}

// extractCtidShard reads one block range, stamping every row Coarse into the shard's
// writer (so the tracker ack-counts completion), then drains the writer on a clean read.
// A row-limit truncation suppresses the marker so the shard stays resumable.
func (s *Source) extractCtidShard(ctx context.Context, sink arrowbatch.Inlet, q querier, sh shard, part, limit int) error {
	w, err := sink.Builder(sh.table, part, sh.dec.schema)
	if err != nil {
		return err
	}
	n, err := s.readBlocks(ctx, w, q, sh, "", nil, rowmodel.Meta{Coarse: true}, limit)
	if err != nil {
		return err
	}
	if limit > 0 && n >= limit {
		return nil // truncated: no completion marker, shard stays resumable
	}
	return w.Drain(rowmodel.Meta{})
}
