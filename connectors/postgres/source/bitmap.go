package postgres

// Bitmap sub-range read mode (docs/resume-read-modes-plan.md). The key space is split into
// sub-ranges (the same sampled boundaries the keyset split uses); each is read with a plain
// range predicate and NO ORDER BY / LIMIT, so the planner is free to pick a bitmap heap
// scan — index TIDs collected, sorted, heap visited in ascending block order with prefetch.
// That recovers the random-key I/O penalty without ordered delivery, which the idempotent
// PK-upsert sink never needed.
//
// Because reads are unordered there is no monotonic per-row cursor. Resume is therefore
// all-or-nothing per sub-range: each row is stamped Coarse, and after a sub-range drains the
// reader pushes a Drained sentinel. The pipeline counts written rows per sub-range against
// that sentinel's total and flags the shard Done only when every row is written — safe under
// parallel writers. Resume skips Done shards and re-reads the rest whole.

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament"
)

// resolveMode picks the read mode for a table at plan time from the source's read_mode
// config: "bitmap" forces it, "ctid" forces ctid+xmin, "auto" runs the correlation probe
// (only bitmap and keyset verdicts act on correlation today — ctid+xmin requires explicit
// opt-in via PKMutable), and anything else stays keyset.
func (s *Source) resolveMode(ctx context.Context, table string) ReadMode {
	switch s.readMode {
	case checkpoint.ModeBitmap:
		return ModeBitmap
	case checkpoint.ModeCtid:
		return ModeCtidXmin
	case "auto":
		// Only the bitmap verdict is actionable today; the physical tiers route in
		// explicitly (ctid+xmin needs a PK-mutable / as-of-completion declaration).
		if m, _, _ := s.Route(ctx, table, RouteSignals{}); m == ModeBitmap {
			return ModeBitmap
		}
		return ModeKeyset
	default:
		return ModeKeyset
	}
}

// planBitmap lays out a fresh bitmap checkpoint: sampled (or arithmetic, for an integer
// leading key) sub-range boundaries, same shape as the keyset split but Mode=bitmap and
// resumed via per-shard Done. A table too small to split, or a sampler failure, becomes a
// single open shard (one full bitmap/seq scan — still correct, coarser resume).
func (s *Source) planBitmap(ctx context.Context, table string, pks []pkColumn) (checkpoint.KeysetCheckpoint, error) {
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	ks := checkpoint.KeysetCheckpoint{Mode: checkpoint.ModeBitmap, Cols: pkNames(pks), Types: pkTypes(pks)}
	k := s.keyShardCount(ctx, qualified)

	var bounds []string
	if k > 1 {
		if isIntType(pks[0].typ) {
			if lo, hi, ok, err := s.intMinMax(ctx, qualified, pks[0].name); err == nil && ok {
				bounds = intBoundaries(lo, hi, k)
			}
		} else if b, err := s.sampleBoundaries(ctx, qualified, pks[0].name, k); err == nil {
			bounds = b
		}
	}
	ks.Shards = splitFromBoundaries(bounds)
	return ks, nil
}

// extractBitmapShard reads one sub-range with a bound-only predicate and no ORDER BY/LIMIT,
// streaming every row stamped Coarse with the shard ordinal. On a clean full drain it pushes
// a Drained sentinel so the shard can be marked complete; if a row limit truncated the read
// it does NOT (an incomplete shard must stay resumable).
func (s *Source) extractBitmapShard(ctx context.Context, sink ingestion.RecordSink, q querier, sh keyShard, limit int) error {
	idSel := keysetIDExpr(sh.pks)
	where, args := bitmapWhere(sh)
	sql := fmt.Sprintf("SELECT %s AS id, to_jsonb(t)::text AS data FROM %s t%s", idSel, sh.qualified, where)

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("bitmap %q: %w", sh.table, err)
	}
	emitted := 0
	truncated := false
	var id string
	var data []byte
	dest := []any{&id, &data}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return fmt.Errorf("bitmap scan %q: %w", sh.table, err)
		}
		rec := ingestion.NewRecord(sh.table, id, append([]byte(nil), data...))
		rec.Part = sh.part
		rec.Coarse = true
		if err := sink.Push(rec); err != nil {
			rows.Close()
			return err
		}
		emitted++
		if limit > 0 && emitted >= limit {
			truncated = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if truncated {
		return nil // partial shard: no completion marker, so resume re-reads it
	}
	// Completion sentinel: the pipeline turns it into this shard's expected-row count.
	return sink.Push(ingestion.Record{Resource: sh.table, Part: sh.part, Coarse: true, Drained: true})
}

// bitmapWhere builds the sub-range predicate from the shard's leading-column bounds only —
// no cursor, no ORDER BY — leaving the planner free to choose a bitmap heap scan. Bounds may
// be a prefix of a composite key; they compare the first len(bound) columns.
func bitmapWhere(sh keyShard) (string, []any) {
	var clauses []string
	var args []any
	n := 1
	if len(sh.lo) > 0 {
		c, a, nn := tupleCmp(sh.pks[:len(sh.lo)], ">=", sh.lo, n)
		clauses, args, n = append(clauses, c), append(args, a...), nn
	}
	if len(sh.hi) > 0 {
		c, a, nn := tupleCmp(sh.pks[:len(sh.hi)], "<", sh.hi, n)
		clauses, args, n = append(clauses, c), append(args, a...), nn
	}
	_ = n
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
