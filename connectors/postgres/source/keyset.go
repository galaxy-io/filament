package postgres

// Resumable full-snapshot reads via a stable primary-key keyset (WHERE pk > cursor
// ORDER BY pk LIMIT page) instead of physical ctid windows. A ctid is not stable across
// runs/snapshots so it cannot be a resume cursor; the primary key is. Each page is an
// index range scan bounded by ORDER BY + LIMIT, so it never re-scans to end-of-table
// (the trap an open-ended ctid scan falls into) — it stays linear in rows read.
//
// A table with a single integer primary key is split into PK ranges so it keeps
// intra-table parallelism; other keyed tables read as one open-ended shard; a keyless
// table cannot be resumed and falls back to the ctid reader (re-read whole on resume,
// made harmless by the idempotent upsert sink). Each shard carries its own cursor in
// the resource checkpoint (see pkg KeysetCheckpoint), so resume re-reads only what each
// shard had not yet delivered.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

// maxKeysetShards caps PK-range fan-out per table, mirroring the ctid reader's cap.
const maxKeysetShards = maxShardsPerTable

// keyShard is one resumable slice of a table's primary-key space: the half-open key
// range [lo, hi) (nil = open), the shard ordinal stamped on every record, and the seed
// cursor to resume from (nil = start at lo).
type keyShard struct {
	table     string
	qualified string
	pks       []pkColumn
	dec       *rowDecoder
	part      int
	lo, hi    []string
	seed      []string
}

// PlanResume builds the per-resource keyset plan: the shard layout plus any cursor
// carried over from prev. A resource without a primary key maps to nil (non-resumable;
// read whole via ctid). The engine persists this plan so a later resume reuses the same
// stable shard boundaries.
func (s *Source) PlanResume(ctx context.Context, resources []string, prev map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	plan := make(map[string]filament.Checkpoint, len(resources))
	for _, table := range resources {
		pks, err := s.lookupPrimaryKey(ctx, s.schema, table)
		if err != nil {
			return nil, fmt.Errorf("lookup pk %q: %w", table, err)
		}
		if len(pks) == 0 {
			plan[table] = nil // no key → not resumable
			continue
		}
		// Resume: reuse the persisted layout verbatim (stable boundaries, advanced keys,
		// completed-shard Done flags, and the original mode) — unless a ctid plan fails its
		// rewrite guard, in which case the physical cursor is stale and the table re-plans.
		if prevKs, ok := checkpoint.ParseKeyset(prev[table]); ok && len(prevKs.Shards) > 0 {
			if intact, err := s.ctidGuardIntact(ctx, table, prevKs); err != nil {
				return nil, err
			} else if intact {
				plan[table] = prevKs.ToCheckpoint(table)
				continue
			}
			// guard failed → fall through and re-plan from scratch (whole table re-read)
		}
		var ks checkpoint.KeysetCheckpoint
		var err2 error
		switch s.resolveMode(ctx, table) {
		case ModeBitmap:
			ks = s.planBitmap(ctx, table, pks)
		case ModeCtidXmin:
			ks, err2 = s.planCtid(ctx, table, pks)
		default:
			ks, err2 = s.planKeyset(ctx, table, pks)
		}
		if err2 != nil {
			return nil, err2
		}
		plan[table] = ks.ToCheckpoint(table)
	}
	return plan, nil
}

// planKeyset lays out a fresh (first-run) keyset checkpoint. When the LEADING primary-key
// column is an integer the table splits into ranges over that column for parallelism —
// the bound is a prefix of the key, but since the PK index is ordered leading-first, a
// range on the leading column is still a contiguous index slice. This covers a composite
// key like lineitem(l_orderkey, l_linenumber): shard by l_orderkey, page by the full
// tuple. Any other key (non-integer leading column) is one open-ended shard. The bottom
// shard is open below and the top open above, so the plan covers the whole key space (and
// any rows appended past the original max) on resume.
func (s *Source) planKeyset(ctx context.Context, table string, pks []pkColumn) (checkpoint.KeysetCheckpoint, error) {
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	ks := checkpoint.KeysetCheckpoint{Cols: pkNames(pks), Types: pkTypes(pks)}
	k := s.keyShardCount(ctx, qualified)

	if isIntType(pks[0].typ) {
		lo, hi, ok, err := s.intMinMax(ctx, qualified, pks[0].name) // leading column only
		if err != nil {
			return ks, fmt.Errorf("min/max %q: %w", table, err)
		}
		if ok {
			ks.Shards = splitIntRange(lo, hi, k) // free arithmetic split (assumes uniform keys)
			return ks, nil
		}
	} else if k > 1 {
		// Non-integer leading key (uuid, text, timestamp): sample equal-count boundaries so
		// the table parallelizes like an integer one. Sampler failure or too few distinct
		// boundaries degrades to a single open shard — never fail a run over a plan heuristic.
		if bounds, err := s.sampleBoundaries(ctx, qualified, pks[0].name, k); err == nil && len(bounds) > 0 {
			ks.Shards = splitFromBoundaries(bounds)
			return ks, nil
		}
	}
	ks.Shards = []checkpoint.KeysetShard{{}} // one open-ended shard
	return ks, nil
}

// sampleBoundaries returns k-1 ordered, deduped leading-column values splitting the table
// into ~equal-count ranges, via percentile_disc (actual existing values at evenly-spaced
// quantiles — better balance than the integer arithmetic split, which assumes a uniform
// distribution). One index-ordered scan of one column, paid once at plan time and frozen
// into the persisted checkpoint. k<=1 or an empty table returns nil (caller single-shards).
func (s *Source) sampleBoundaries(ctx context.Context, qualified, col string, k int) ([]string, error) {
	if k <= 1 {
		return nil, nil
	}
	fracs := make([]string, 0, k-1)
	for i := 1; i < k; i++ {
		fracs = append(fracs, strconv.FormatFloat(float64(i)/float64(k), 'f', -1, 64))
	}
	c := pgx.Identifier{col}.Sanitize()
	q := fmt.Sprintf("SELECT percentile_disc(ARRAY[%s]) WITHIN GROUP (ORDER BY %s)::text[] FROM %s",
		strings.Join(fracs, ","), c, qualified)
	var arr []string
	if err := s.pool.QueryRow(ctx, q).Scan(&arr); err != nil {
		return nil, err
	}
	return dedupeOrdered(arr), nil
}

// dedupeOrdered drops nils and collapses adjacent duplicates from an already-ordered slice.
// A low-cardinality leading column repeats quantile values → empty shards (harmless but
// wasteful), so duplicates are removed; the result is strictly increasing.
func dedupeOrdered(vals []string) []string {
	out := vals[:0:0]
	for _, v := range vals {
		if v == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == v {
			continue
		}
		out = append(out, v)
	}
	return out
}

// ExtractFrom reads each resource from its checkpoint; the checkpoint plan's
// mode decides how. Incremental plans read by cursor, everything else reads
// via keyset shards (concurrently, under one exported snapshot, like Extract);
// a resource with no keyset plan (no primary key, or a checkpoint-free full
// read) falls back to the ctid reader and is read whole.
func (s *Source) ExtractFrom(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts, prev map[string]filament.Checkpoint) error {
	var incremental, rest []string
	for _, table := range opts.Resources {
		if plan, ok := checkpoint.ParseKeyset(prev[table]); ok && plan.Mode == checkpoint.ModeIncremental {
			incremental = append(incremental, table)
		} else {
			rest = append(rest, table)
		}
	}
	if len(incremental) > 0 {
		incrementalOpts := opts
		incrementalOpts.Resources = incremental
		if err := s.extractIncremental(ctx, sink, incrementalOpts, prev); err != nil {
			return err
		}
	}
	opts.Resources = rest
	var jobs []func(context.Context, querier) error
	for _, table := range opts.Resources {
		ks, ok := checkpoint.ParseKeyset(prev[table])
		if !ok || len(ks.Cols) == 0 {
			// No keyset plan → ctid fallback for this table (non-resumable).
			shards, err := s.planShards(ctx, []string{table}, opts.Parallelism)
			if err != nil {
				return err
			}
			for _, sh := range shards {
				jobs = append(jobs, func(ctx context.Context, q querier) error {
					err := s.extractShard(ctx, sink, q, sh, opts.Limit)
					return err
				})
			}
			continue
		}
		qualified := pgx.Identifier{s.schema, table}.Sanitize()
		dec, err := s.decoderFor(ctx, table, ks.Cols)
		if err != nil {
			return err
		}
		if ks.Mode == checkpoint.ModeCtid {
			jobs = append(jobs, s.ctidJobs(ctx, sink, table, qualified, ks, dec, opts.Limit)...)
			continue
		}
		bitmap := ks.Mode == checkpoint.ModeBitmap
		for i, ksh := range keyShardsFrom(table, qualified, ks, dec) {
			if bitmap {
				if ks.Shards[i].Done {
					continue // already fully read+written in a prior run
				}
				jobs = append(jobs, func(ctx context.Context, q querier) error {
					return s.extractBitmapShard(ctx, sink, q, ksh, opts.Limit)
				})
				continue
			}
			jobs = append(jobs, func(ctx context.Context, q querier) error {
				return s.extractKeysetShard(ctx, sink, q, ksh, opts.Limit)
			})
		}
	}
	return s.runConcurrent(ctx, opts.Parallelism, jobs)
}

// keyShardsFrom turns a decoded checkpoint into the table's runnable shards, seeding
// each with its persisted cursor.
func keyShardsFrom(table, qualified string, ks checkpoint.KeysetCheckpoint, dec *rowDecoder) []keyShard {
	pks := make([]pkColumn, len(ks.Cols))
	for i := range ks.Cols {
		pks[i] = pkColumn{name: ks.Cols[i], typ: typeAt(ks.Types, i)}
	}
	out := make([]keyShard, len(ks.Shards))
	for i, sh := range ks.Shards {
		out[i] = keyShard{table: table, qualified: qualified, pks: pks, dec: dec, part: i, lo: sh.Lo, hi: sh.Hi, seed: sh.Key}
	}
	return out
}

// extractKeysetShard pages one shard out through the sink: WHERE (pk-tuple) is above the
// cursor (or the shard's lower bound) and below the shard's upper bound, ORDER BY the key,
// LIMIT a page. Every row carries its key so the pipeline can carry the cursor forward.
func (s *Source) extractKeysetShard(ctx context.Context, sink filament.RecordSink, q querier, sh keyShard, limit int) error {
	w, err := sink.Builder(sh.table, sh.part, sh.dec.schema)
	if err != nil {
		return err
	}
	order := keysetOrder(sh.pks)
	cur := sh.seed
	emitted := 0
	for {
		where, args := keysetWhere(sh, cur)
		sql := fmt.Sprintf("SELECT %s FROM %s t%s ORDER BY %s LIMIT %d",
			sh.dec.selectList, sh.qualified, where, order, s.pageSize)
		n, last, err := s.appendPage(ctx, q, sql, w, sh.dec, filament.RowMeta{}, sh.dec.pkIdx, remaining(limit, emitted), args...)
		if err != nil {
			return fmt.Errorf("keyset %q: %w", sh.table, err)
		}
		emitted += n
		if limit > 0 && emitted >= limit {
			return nil
		}
		if n < s.pageSize {
			return nil // shard exhausted
		}
		cur = last
	}
}

// keysetWhere builds the page predicate and its bound args. The lower bound is the cursor
// (the full key tuple, strictly greater) once paging has started, else the shard's
// inclusive lower bound; the upper bound is the shard's exclusive upper bound. Shard
// bounds may be a PREFIX of the key (a leading-column range split), so they compare only
// the first len(bound) columns; the cursor always compares the whole tuple. All are text
// values cast to the key's column types.
func keysetWhere(sh keyShard, cur []string) (string, []any) {
	var clauses []string
	var args []any
	n := 1
	// A bound is present only when it carries a key value; an empty/nil slice means
	// "open" (start of the key space / no upper limit). The cursor must be the full tuple.
	if len(cur) == len(sh.pks) {
		c, a, nn := tupleCmp(sh.pks, ">", cur, n)
		clauses, args, n = append(clauses, c), append(args, a...), nn
	} else if len(sh.lo) > 0 {
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

// tupleCmp renders a row comparison `(t."a", t."b") op ($1::ta, $2::tb)` (or the scalar
// form for a single column) and the matching args, starting placeholders at argStart.
func tupleCmp(pks []pkColumn, op string, vals []string, argStart int) (string, []any, int) {
	cols := make([]string, len(pks))
	ph := make([]string, len(pks))
	args := make([]any, len(pks))
	for i, pk := range pks {
		cols[i] = "t." + pgx.Identifier{pk.name}.Sanitize()
		ph[i] = fmt.Sprintf("$%d::%s", argStart+i, pk.typ)
		args[i] = vals[i]
	}
	if len(pks) == 1 {
		return cols[0] + " " + op + " " + ph[0], args, argStart + 1
	}
	return "(" + strings.Join(cols, ", ") + ") " + op + " (" + strings.Join(ph, ", ") + ")", args, argStart + len(pks)
}

// keysetOrder is the ORDER BY over the key columns (so each page is an index range scan).
func keysetOrder(pks []pkColumn) string {
	parts := make([]string, len(pks))
	for i, pk := range pks {
		parts[i] = "t." + pgx.Identifier{pk.name}.Sanitize()
	}
	return strings.Join(parts, ", ")
}

// runConcurrent runs jobs over the pool sequentially, or — when parallel and more than
// one job — concurrently, each in its own transaction importing one shared exported
// snapshot, so every shard sees a single consistent point-in-time (the Extract pattern).
func (s *Source) runConcurrent(ctx context.Context, parallelism int, jobs []func(context.Context, querier) error) error {
	if parallelism <= 1 || len(jobs) <= 1 {
		for _, job := range jobs {
			if err := job(ctx, s.pool); err != nil {
				return err
			}
		}
		return nil
	}

	snap, err := s.openSnapshot(ctx)
	if err != nil {
		return err
	}
	defer snap.close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error
	for _, job := range jobs {
		wg.Add(1)
		go func(job func(context.Context, querier) error) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if err := s.withSnapshotTx(ctx, snap, job); err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(job)
	}
	wg.Wait()
	return firstErr
}

// withSnapshotTx runs job inside a read-only transaction that imports the run's exported
// snapshot.
func (s *Source) withSnapshotTx(ctx context.Context, snap *snapshot, job func(context.Context, querier) error) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire shard conn: %w", err)
	}
	defer conn.Release()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin shard tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET TRANSACTION SNAPSHOT "+quoteLiteral(snap.id)); err != nil {
		return fmt.Errorf("import snapshot: %w", err)
	}
	return job(ctx, tx)
}

// intMinMax returns the integer key range of the table as int64 bounds, ok=false when
// the table is empty.
func (s *Source) intMinMax(ctx context.Context, qualified, col string) (int64, int64, bool, error) {
	c := pgx.Identifier{col}.Sanitize()
	q := fmt.Sprintf("SELECT min(%s), max(%s) FROM %s", c, c, qualified)
	var lo, hi *int64
	if err := s.pool.QueryRow(ctx, q).Scan(&lo, &hi); err != nil {
		return 0, 0, false, err
	}
	if lo == nil || hi == nil {
		return 0, 0, false, nil
	}
	return *lo, *hi, true, nil
}

// keyShardCount sizes a table's PK split by heap blocks, like the ctid reader, bounded by
// maxKeysetShards. Returns 1 when sharding is disabled.
func (s *Source) keyShardCount(ctx context.Context, qualified string) int {
	if s.shardPages <= 0 {
		return 1
	}
	pages, err := s.lookupPages(ctx, qualified)
	if err != nil {
		return 1
	}
	k := max((pages+s.shardPages-1)/s.shardPages, 1)
	return min(k, maxKeysetShards)
}

// splitIntRange divides [lo, hi] into k contiguous integer shards by producing k-1
// arithmetic boundaries and handing them to splitFromBoundaries (shared with the sampled
// non-integer path). Assumes a roughly uniform key distribution — free (no scan), which is
// why it stays the integer fast path rather than sampling.
func splitIntRange(lo, hi int64, k int) []checkpoint.KeysetShard {
	return splitFromBoundaries(intBoundaries(lo, hi, k))
}

// intBoundaries produces the k-1 arithmetic split points of [lo, hi]; empty for k<=1 or an
// inverted/degenerate range (caller single-shards).
func intBoundaries(lo, hi int64, k int) []string {
	if k <= 1 || hi <= lo {
		return nil
	}
	span := hi - lo + 1
	bounds := make([]string, 0, k-1)
	for i := 1; i < k; i++ {
		bounds = append(bounds, strconv.FormatInt(lo+int64(i)*span/int64(k), 10))
	}
	return bounds
}

// splitFromBoundaries builds k shards from k-1 ordered leading-column boundaries:
// {nil,b0}, {b0,b1}, …, {b_{k-2},nil}. The bottom shard is open below and the top open
// above, so the layout covers any row — including ones appended past the max boundary
// before a resume. Bounds are a prefix of the key (the leading column); keysetWhere
// compares them as such.
func splitFromBoundaries(bounds []string) []checkpoint.KeysetShard {
	if len(bounds) == 0 {
		return []checkpoint.KeysetShard{{}}
	}
	shards := make([]checkpoint.KeysetShard, 0, len(bounds)+1)
	var prev []string
	for _, b := range bounds {
		shards = append(shards, checkpoint.KeysetShard{Lo: prev, Hi: []string{b}})
		prev = []string{b}
	}
	return append(shards, checkpoint.KeysetShard{Lo: prev, Hi: nil})
}

// isIntType reports whether a Postgres type is an integer the splitter can range over.
func isIntType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "smallint", "integer", "bigint", "int2", "int4", "int8":
		return true
	default:
		return false
	}
}

func pkNames(pks []pkColumn) []string {
	out := make([]string, len(pks))
	for i, pk := range pks {
		out[i] = pk.name
	}
	return out
}

func pkTypes(pks []pkColumn) []string {
	out := make([]string, len(pks))
	for i, pk := range pks {
		out[i] = pk.typ
	}
	return out
}

func typeAt(types []string, i int) string {
	if i < len(types) {
		return types[i]
	}
	return "text"
}
