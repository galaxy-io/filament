package mysql

// Resumable full-snapshot reads via a stable primary-key keyset (WHERE pk > cursor
// ORDER BY pk LIMIT page). On InnoDB the primary key is the clustered index, so key
// order is physical order: every page is a bounded range scan of contiguous leaf
// pages, sequential I/O with a cursor that is also stable across runs (the primary
// key, unlike a physical row address, survives vacuuming and reorganization).
//
// A table with an integer leading primary-key column is split into key ranges by
// arithmetic over min/max; other keyed tables split by sampled equal-count
// boundaries; a keyless table cannot be resumed and is re-read whole (made harmless
// by the idempotent upsert sink). Each shard carries its own cursor in the resource
// checkpoint (see pkg checkpoint), so resume re-reads only what each shard had not
// yet delivered.

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

// maxKeysetShards caps PK-range fan-out per table, mirroring the Postgres reader's cap.
const maxKeysetShards = maxShardsPerTable

// keysetPlan is a decoded per-resource shard layout; aliased so the planner and the
// fresh-extract path share one shape.
type keysetPlan = checkpoint.KeysetCheckpoint

// keyShard is one resumable slice of a table's primary-key space: the half-open key
// range [lo, hi) (nil = open), the shard ordinal stamped on every record, and the seed
// cursor to resume from (nil = start at lo).
type keyShard struct {
	table     string
	qualified string
	jsonExpr  string
	pks       []pkColumn
	part      int
	lo, hi    []string
	seed      []string
}

// PlanResume builds the per-resource keyset plan: the shard layout plus any cursor
// carried over from prev. A resource without a primary key maps to nil (non-resumable;
// re-read whole). The engine persists this plan so a later resume reuses the same
// stable shard boundaries.
func (s *Source) PlanResume(ctx context.Context, resources []string, prev map[string]ingestion.Checkpoint) (map[string]ingestion.Checkpoint, error) {
	plan := make(map[string]ingestion.Checkpoint, len(resources))
	for _, table := range resources {
		_, pks, err := s.tableMeta(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("lookup pk %q: %w", table, err)
		}
		if len(pks) == 0 {
			plan[table] = nil // no key → not resumable
			continue
		}
		// Resume: reuse the persisted layout verbatim (stable boundaries and advanced
		// keys) — the keyset cursor is a key-space position, valid across runs.
		if prevKs, ok := checkpoint.ParseKeyset(prev[table]); ok && len(prevKs.Shards) > 0 {
			plan[table] = prevKs.ToCheckpoint(table)
			continue
		}
		qualified := quoteIdent(s.database) + "." + quoteIdent(table)
		ks, err := s.planKeyset(ctx, table, qualified, pks)
		if err != nil {
			return nil, err
		}
		plan[table] = ks.ToCheckpoint(table)
	}
	return plan, nil
}

// planKeyset lays out a fresh (first-run) keyset checkpoint. When the LEADING
// primary-key column is an integer the table splits into ranges over that column for
// parallelism — the bound is a prefix of the key, but since the clustered index is
// ordered leading-first, a range on the leading column is still a contiguous index
// slice. Any other leading key samples equal-count boundaries. The bottom shard is
// open below and the top open above, so the plan covers the whole key space (and any
// rows appended past the original max) on resume.
func (s *Source) planKeyset(ctx context.Context, table, qualified string, pks []pkColumn) (keysetPlan, error) {
	ks := keysetPlan{Cols: pkNames(pks), Types: pkTypes(pks)}
	k := s.keyShardCount(ctx, table)

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
		// Non-integer leading key (uuid, varchar, datetime): sample equal-count
		// boundaries so the table parallelizes like an integer one. Sampler failure or
		// too few distinct boundaries degrades to a single open shard — never fail a
		// run over a plan heuristic.
		if bounds, err := s.sampleBoundaries(ctx, qualified, pks[0].name, k); err == nil && len(bounds) > 0 {
			ks.Shards = splitFromBoundaries(bounds)
			return ks, nil
		}
	}
	ks.Shards = []checkpoint.KeysetShard{{}} // one open-ended shard
	return ks, nil
}

// sampleBoundaries returns up to k-1 ordered, deduped leading-column values splitting
// the table into ~equal-count ranges. MySQL has no percentile_disc, so this is two
// index-only queries: a row count, then the values at the evenly-spaced row-number
// positions (one ordered scan of the leading key column, paid once at plan time and
// frozen into the persisted checkpoint). Drift between the two queries under
// concurrent writes only skews shard balance, never correctness.
func (s *Source) sampleBoundaries(ctx context.Context, qualified, col string, k int) ([]string, error) {
	if k <= 1 {
		return nil, nil
	}
	var n int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qualified).Scan(&n); err != nil {
		return nil, err
	}
	if n < int64(k) {
		return nil, nil
	}
	positions := make([]string, 0, k-1)
	for i := 1; i < k; i++ {
		positions = append(positions, strconv.FormatInt(n*int64(i)/int64(k), 10))
	}
	c := quoteIdent(col)
	q := fmt.Sprintf(
		"SELECT CAST(v AS CHAR) FROM (SELECT %s AS v, ROW_NUMBER() OVER (ORDER BY %s) AS rn FROM %s) d WHERE d.rn IN (%s) ORDER BY d.rn",
		c, c, qualified, strings.Join(positions, ","))
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bounds []string
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if v.Valid {
			bounds = append(bounds, v.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dedupeOrdered(bounds), nil
}

// dedupeOrdered drops empties and collapses adjacent duplicates from an already-ordered
// slice. A low-cardinality leading column repeats quantile values → empty shards
// (harmless but wasteful), so duplicates are removed; the result is strictly increasing.
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

// ExtractFrom reads each resource from its checkpoint. Keyed resources read via keyset
// shards (concurrently, like Extract); a resource with no keyset plan (no primary key)
// falls back to the streaming full scan and is re-read whole.
func (s *Source) ExtractFrom(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts, prev map[string]ingestion.Checkpoint) error {
	var jobs []func(context.Context, querier) error
	for _, table := range opts.Resources {
		var plan *keysetPlan
		if ks, ok := checkpoint.ParseKeyset(prev[table]); ok && len(ks.Cols) > 0 {
			plan = &ks
		}
		tjobs, err := s.tableJobs(ctx, sink, table, plan, opts.Limit)
		if err != nil {
			return err
		}
		jobs = append(jobs, tjobs...)
	}
	return s.runConcurrent(ctx, opts.Parallelism, jobs)
}

// keyShardsFrom turns a plan into the table's runnable shards, seeding each with its
// persisted cursor.
func keyShardsFrom(table, qualified, jsonExpr string, pks []pkColumn, ks keysetPlan) []keyShard {
	// Prefer the plan's column/type list (frozen at plan time) over the live catalog,
	// so a resumed run pages by exactly the key the boundaries were laid out over.
	if len(ks.Cols) > 0 {
		pks = make([]pkColumn, len(ks.Cols))
		for i := range ks.Cols {
			pks[i] = pkColumn{name: ks.Cols[i], typ: typeAt(ks.Types, i)}
		}
	}
	out := make([]keyShard, len(ks.Shards))
	for i, sh := range ks.Shards {
		out[i] = keyShard{table: table, qualified: qualified, jsonExpr: jsonExpr, pks: pks, part: i, lo: sh.Lo, hi: sh.Hi, seed: sh.Key}
	}
	return out
}

// extractKeysetShard pages one shard out through the sink: WHERE (pk-tuple) is above
// the cursor (or the shard's lower bound) and below the shard's upper bound, ORDER BY
// the key, LIMIT a page. Every record is stamped with the shard ordinal and its key so
// the pipeline can carry the cursor forward. Each page is buffered and the result set
// closed before any Push (a push can block on backpressure while holding a connection).
func (s *Source) extractKeysetShard(ctx context.Context, sink ingestion.RecordSink, q querier, sh keyShard, limit int) error {
	idSel := keysetIDExpr(sh.pks)
	keySel, keyCols := keysetKeyProjection(sh.pks)
	order := keysetOrder(sh.pks)

	// Every page after the first filters by the full-tuple cursor, so its SQL
	// text is constant across the whole shard: prepare it once and reuse it.
	// database/sql otherwise re-prepares (prepare → execute → close, three
	// round trips) on every parameterized QueryContext — at page_size rows per
	// page that overhead dominates a large table's read.
	var cursorStmt *sql.Stmt
	defer func() {
		if cursorStmt != nil {
			_ = cursorStmt.Close()
		}
	}()

	cur := sh.seed
	emitted := 0
	for {
		where, args := keysetWhere(sh, cur)
		query := fmt.Sprintf("SELECT %s AS id, %s AS data, %s FROM %s t%s ORDER BY %s LIMIT %d",
			idSel, sh.jsonExpr, keySel, sh.qualified, where, order, s.pageSize)

		var rows *sql.Rows
		var err error
		if len(cur) == len(sh.pks) { // cursor form — constant SQL, reusable stmt
			if cursorStmt == nil {
				if cursorStmt, err = q.PrepareContext(ctx, query); err != nil {
					return fmt.Errorf("keyset prepare %q: %w", sh.table, err)
				}
			}
			rows, err = cursorStmt.QueryContext(ctx, args...)
		} else { // first page only (shard bound / fresh start)
			rows, err = q.QueryContext(ctx, query, args...)
		}
		if err != nil {
			return fmt.Errorf("keyset %q: %w", sh.table, err)
		}
		page, err := s.readKeysetPage(rows, sh, keyCols)
		if err != nil {
			return fmt.Errorf("keyset %q: %w", sh.table, err)
		}
		for _, rec := range page {
			if err := sink.Push(rec); err != nil {
				return err
			}
			emitted++
			if limit > 0 && emitted >= limit {
				return nil
			}
		}
		if len(page) < s.pageSize {
			return nil // shard exhausted
		}
		cur = page[len(page)-1].Key
	}
}

// readKeysetPage drains one page's rows into records, stamped with shard part
// and key tuple. The result set is closed before returning so the connection
// is free before records are pushed.
func (s *Source) readKeysetPage(rows *sql.Rows, sh keyShard, keyCols int) ([]ingestion.Record, error) {
	defer rows.Close()

	out := make([]ingestion.Record, 0, s.pageSize)
	// Scan destinations are hoisted and reused: database/sql clones the driver's
	// buffer into freshly allocated id/data/keys values on every row (so each
	// Record keeps its own backing), while the dest slice is allocated once for
	// the whole page instead of per row.
	var (
		id   string
		data []byte
	)
	keys := make([]string, keyCols)
	dest := make([]any, 2+keyCols)
	dest[0], dest[1] = &id, &data
	for i := range keys {
		dest[2+i] = &keys[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rec := ingestion.NewRecord(sh.table, id, data)
		rec.Part = sh.part
		rec.Key = append([]string(nil), keys...)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// keysetWhere builds the page predicate and its bound args. The lower bound is the
// cursor (the full key tuple, strictly greater) once paging has started, else the
// shard's inclusive lower bound; the upper bound is the shard's exclusive upper bound.
// Shard bounds may be a PREFIX of the key (a leading-column range split), so they
// compare only the first len(bound) columns; the cursor always compares the whole
// tuple. All are text values cast back to the key's column types.
func keysetWhere(sh keyShard, cur []string) (string, []any) {
	var clauses []string
	var args []any
	if len(cur) == len(sh.pks) {
		c, a := tupleCmp(sh.pks, ">", cur)
		clauses, args = append(clauses, c), append(args, a...)
	} else if len(sh.lo) > 0 {
		c, a := tupleCmp(sh.pks[:len(sh.lo)], ">=", sh.lo)
		clauses, args = append(clauses, c), append(args, a...)
	}
	if len(sh.hi) > 0 {
		c, a := tupleCmp(sh.pks[:len(sh.hi)], "<", sh.hi)
		clauses, args = append(clauses, c), append(args, a...)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// tupleCmp renders a lexicographic key comparison in MySQL's seek-optimizable
// expanded form and returns the matching args. MySQL's range optimizer does not
// use a row constructor `(a, b) > (?, ?)` for index access: that plan degrades to
// a full index scan + filter, so every page re-reads and discards all rows before
// the cursor and a table read goes quadratic. The disjunctive expansion
// `(a > ? OR (a = ? AND b > ?))` gets a covering index range scan that seeks
// straight to the cursor. Each non-final column's value binds twice (strict
// compare + equality); only the final column applies op's equality part (for
// >= / < prefix bounds).
func tupleCmp(pks []pkColumn, op string, vals []string) (string, []any) {
	strict := strings.TrimSuffix(op, "=")
	var args []any
	var expand func(i int) string
	expand = func(i int) string {
		col := "t." + quoteIdent(pks[i].name)
		ph := castExpr(pks[i].typ)
		if i == len(pks)-1 {
			args = append(args, vals[i])
			return col + " " + op + " " + ph
		}
		args = append(args, vals[i], vals[i])
		return "(" + col + " " + strict + " " + ph + " OR (" + col + " = " + ph + " AND " + expand(i+1) + "))"
	}
	return expand(0), args
}

// castExpr renders a placeholder cast back to a key column's type so the comparison
// is typed (and index-usable) instead of string-coerced. MySQL CAST targets differ
// from column type names; anything unmapped compares as a plain string placeholder,
// which is exact for character keys.
func castExpr(dataType string) string {
	switch strings.ToLower(dataType) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return "CAST(? AS SIGNED)"
	case "decimal", "numeric":
		return "CAST(? AS DECIMAL(65,30))"
	case "date":
		return "CAST(? AS DATE)"
	case "datetime", "timestamp":
		return "CAST(? AS DATETIME(6))"
	case "time":
		return "CAST(? AS TIME(6))"
	case "binary", "varbinary":
		return "CAST(? AS BINARY)"
	default:
		return "?"
	}
}

// keysetIDExpr derives the record id from the key columns: the single value as text,
// or composite columns joined with CHAR(31) — a separator that cannot appear in
// normal keys, matching the Postgres reader's convention.
func keysetIDExpr(pks []pkColumn) string {
	parts := make([]string, len(pks))
	for i, pk := range pks {
		parts[i] = "CAST(t." + quoteIdent(pk.name) + " AS CHAR)"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "CONCAT_WS(CHAR(31), " + strings.Join(parts, ", ") + ")"
}

// keysetKeyProjection projects each key column as text (k0, k1, …) so the cursor can
// be read back from each row; returns the SELECT fragment and the column count.
func keysetKeyProjection(pks []pkColumn) (string, int) {
	parts := make([]string, len(pks))
	for i, pk := range pks {
		parts[i] = fmt.Sprintf("CAST(t.%s AS CHAR) AS k%d", quoteIdent(pk.name), i)
	}
	return strings.Join(parts, ", "), len(pks)
}

// keysetOrder is the ORDER BY over the key columns (each page is a clustered-index
// range scan).
func keysetOrder(pks []pkColumn) string {
	parts := make([]string, len(pks))
	for i, pk := range pks {
		parts[i] = "t." + quoteIdent(pk.name)
	}
	return strings.Join(parts, ", ")
}

// runConcurrent runs jobs over the pool based on the parallelism config, if 0 it runs sequentially
// each inside its own REPEATABLE READ transaction opened with START TRANSACTION WITH CONSISTENT SNAPSHOT. 
func (s *Source) runConcurrent(ctx context.Context, parallelism int, jobs []func(context.Context, querier) error) error {
	if parallelism <= 1 || len(jobs) <= 1 {
		for _, job := range jobs {
			if err := s.withSnapshotTx(ctx, job); err != nil {
				return err
			}
		}
		return nil
	}

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
				return // a sibling already failed; stop launching work
			}
			if err := s.withSnapshotTx(ctx, job); err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel() // stop the other readers
				})
			}
		}(job)
	}
	wg.Wait()
	return firstErr
}

// withSnapshotTx runs job on a dedicated connection inside a read-only REPEATABLE READ
// transaction whose snapshot is pinned immediately (WITH CONSISTENT SNAPSHOT), so the
// shard sees one point-in-time regardless of how long it pages.
func (s *Source) withSnapshotTx(ctx context.Context, job func(context.Context, querier) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire shard conn: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		return fmt.Errorf("set isolation: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY"); err != nil {
		return fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK") }()
	return job(ctx, conn)
}

// intMinMax returns the integer key range of the table as int64 bounds, ok=false when
// the table is empty.
func (s *Source) intMinMax(ctx context.Context, qualified, col string) (int64, int64, bool, error) {
	c := quoteIdent(col)
	q := fmt.Sprintf("SELECT MIN(%s), MAX(%s) FROM %s", c, c, qualified)
	var lo, hi sql.NullInt64
	if err := s.db.QueryRowContext(ctx, q).Scan(&lo, &hi); err != nil {
		return 0, 0, false, err
	}
	if !lo.Valid || !hi.Valid {
		return 0, 0, false, nil
	}
	return lo.Int64, hi.Int64, true, nil
}

// keyShardCount sizes a table's PK split by its InnoDB data size in pages (from
// information_schema — an estimate, refreshed by stats, which only skews balance),
// bounded by maxKeysetShards. Returns 1 when sharding is disabled.
func (s *Source) keyShardCount(ctx context.Context, table string) int {
	if s.shardPages <= 0 {
		return 1
	}
	const q = `
SELECT COALESCE(t.DATA_LENGTH, 0) DIV COALESCE((
	SELECT CAST(VARIABLE_VALUE AS UNSIGNED) FROM performance_schema.global_variables
	WHERE VARIABLE_NAME = 'innodb_page_size'
), 16384)
FROM information_schema.TABLES t
WHERE t.TABLE_SCHEMA = ? AND t.TABLE_NAME = ?`
	var pages int
	if err := s.db.QueryRowContext(ctx, q, s.database, table).Scan(&pages); err != nil {
		return 1
	}
	k := max((pages+s.shardPages-1)/s.shardPages, 1)
	return min(k, maxKeysetShards)
}

// splitIntRange divides [lo, hi] into k contiguous integer shards by producing k-1
// arithmetic boundaries and handing them to splitFromBoundaries (shared with the
// sampled non-integer path). Assumes a roughly uniform key distribution — free (no
// scan), which is why it stays the integer fast path rather than sampling.
func splitIntRange(lo, hi int64, k int) []checkpoint.KeysetShard {
	return splitFromBoundaries(intBoundaries(lo, hi, k))
}

// intBoundaries produces the k-1 arithmetic split points of [lo, hi]; empty for k<=1
// or an inverted/degenerate range (caller single-shards).
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

// isIntType reports whether a MySQL DATA_TYPE is an integer the splitter can range over.
func isIntType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
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
	return "char"
}
