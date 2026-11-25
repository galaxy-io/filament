package sqlserver

// Snapshot reads page a table by its primary key. A table splits into shards
// over ranges of the leading key column, each read on its own connection by
// one streaming query, WHERE key >= lo AND key < hi ORDER BY key. The rows
// stream through RawRows as fast as the pipeline takes them, with no paging
// round trips; backpressure simply leaves the server waiting on the network.
//
// Every row carries its key, so the pipeline checkpoints each shard's progress
// and a resumed run starts each shard after the last key it delivered. A
// keyless table has no stable order to resume from and is read whole.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

// maxShardsPerTable caps a table's fan-out.
const maxShardsPerTable = 64

// keyShard is one resumable slice of a table's key space: the half-open
// range [lo, hi) over a prefix of the key (nil = open), its part ordinal and
// the cursor to resume after (nil = start at lo).
type keyShard struct {
	t      *table
	keys   []keyColumn // the plan's key, frozen when the shards were laid out
	part   int
	lo, hi []string
	seed   []string
}

// keyColumn is one key column as a checkpoint records it: its name and
// declared type, which CAST targets when the key is compared.
type keyColumn struct{ name, decl string }

// PlanResume lays out each table's shards, or reuses the layout persisted by
// an interrupted run so its shards resume from their own cursors. A table
// without an ordered key maps to nil and is re-read whole.
func (s *Source) PlanResume(ctx context.Context, resources []string, prev map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	plan := make(map[string]filament.Checkpoint, len(resources))
	for _, name := range resources {
		t, err := s.tableMeta(ctx, name)
		if err != nil {
			return nil, err
		}
		if !t.orderedKey() {
			plan[name] = nil
			continue
		}
		// Shard bounds and cursors are text in the key's declared types. A
		// changed type can order them differently, so only an unchanged key
		// resumes; any other starts over.
		if old, ok := checkpoint.ParseKeyset(prev[name]); ok && len(old.Shards) > 0 &&
			slices.Equal(old.Cols, columnNames(t.pks)) && slices.Equal(old.Types, columnDecls(t.pks)) {
			plan[name] = old.ToCheckpoint(name)
			continue
		}
		ks, err := s.planKeyset(ctx, t)
		if err != nil {
			return nil, err
		}
		plan[name] = ks.ToCheckpoint(name)
	}
	return plan, nil
}

// orderedKey reports whether the table has a primary key whose checkpoint
// text orders as the key does. sql_variant orders by base type before value,
// which its text loses (1 and '1' render alike), so such a table is read
// whole on one connection, as a keyless one is.
func (t *table) orderedKey() bool {
	for _, c := range t.pks {
		if c.typeName == "sql_variant" {
			return false
		}
	}
	return len(t.pks) > 0
}

// planKeyset lays out a fresh plan. An integer leading key splits by
// arithmetic over its range; any other leading key splits at sampled
// equal-count boundaries. The outer shards are open-ended, so the plan covers
// rows inserted beyond the original range before a resume.
func (s *Source) planKeyset(ctx context.Context, t *table) (checkpoint.KeysetCheckpoint, error) {
	ks := checkpoint.KeysetCheckpoint{Cols: columnNames(t.pks), Types: columnDecls(t.pks)}
	k := s.shardCount(ctx, t.qualified)
	if k > 1 {
		lead := t.pks[0]
		if isIntType(lead.typeName) {
			lo, hi, ok, err := s.intRange(ctx, t.qualified, lead.name)
			if err != nil {
				return ks, fmt.Errorf("key range %q: %w", t.name, err)
			}
			if ok {
				ks.Shards = splitFromBoundaries(intBoundaries(lo, hi, k))
				return ks, nil
			}
		} else if bounds, err := s.sampleBoundaries(ctx, t, k); err == nil && len(bounds) > 0 {
			// A failed sample degrades to one shard: never fail a run over a heuristic.
			ks.Shards = splitFromBoundaries(bounds)
			return ks, nil
		}
	}
	ks.Shards = []checkpoint.KeysetShard{{}}
	return ks, nil
}

// shardCount sizes a table's split by its row count, 1 when sharding is off.
func (s *Source) shardCount(ctx context.Context, qualified string) int {
	if s.shardRows <= 0 {
		return 1
	}
	rows, err := s.tableRows(ctx, qualified)
	if err != nil {
		return 1
	}
	k := int(min((rows+s.shardRows-1)/s.shardRows, maxShardsPerTable))
	return max(k, 1)
}

func (s *Source) intRange(ctx context.Context, qualified, col string) (lo, hi int64, ok bool, err error) {
	c := quoteIdent(col)
	var l, h sql.NullInt64
	q := fmt.Sprintf("SELECT CAST(MIN(%s) AS bigint), CAST(MAX(%s) AS bigint) FROM %s", c, c, qualified) //nolint:gosec // quoted identifiers
	if err := s.db.QueryRowContext(ctx, q).Scan(&l, &h); err != nil {
		return 0, 0, false, err
	}
	return l.Int64, h.Int64, l.Valid && h.Valid, nil
}

// sampleBoundaries returns up to k-1 increasing leading-key values that split
// the table into equal-count ranges, in checkpoint text. It reads the leading
// key column once, at plan time.
func (s *Source) sampleBoundaries(ctx context.Context, t *table, k int) ([]string, error) {
	rows, err := s.tableRows(ctx, t.qualified)
	if err != nil || rows < int64(k) {
		return nil, err
	}
	positions := make([]string, 0, k-1)
	for i := 1; i < k; i++ {
		positions = append(positions, strconv.FormatInt(rows*int64(i)/int64(k), 10))
	}
	c := quoteIdent(t.pks[0].name)
	q := fmt.Sprintf( //nolint:gosec // quoted identifiers and formatted integers
		"SELECT v FROM (SELECT %s AS v, ROW_NUMBER() OVER (ORDER BY %s) AS rn FROM %s) d WHERE d.rn IN (%s) ORDER BY d.rn",
		keyValue(t.pks[0].decl(), c), c, t.qualified, strings.Join(positions, ","))
	keys, err := queryKeys(ctx, s.db, q)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, row := range keys {
		if len(out) == 0 || out[len(out)-1] != row[0] {
			out = append(out, row[0])
		}
	}
	return out, nil
}

// intBoundaries produces the k-1 arithmetic split points of [lo, hi].
func intBoundaries(lo, hi int64, k int) []string {
	if k <= 1 || hi <= lo {
		return nil
	}
	// big.Int: i*span overflows int64 for wide keys.
	span := new(big.Int).Sub(big.NewInt(hi), big.NewInt(lo))
	span.Add(span, big.NewInt(1))
	var bounds []string
	for i := 1; i < k; i++ {
		b := new(big.Int).Mul(span, big.NewInt(int64(i)))
		b.Quo(b, big.NewInt(int64(k)))
		b.Add(b, big.NewInt(lo))
		if b.Int64() <= lo {
			continue // would leave the first shard empty
		}
		if s := b.String(); len(bounds) == 0 || bounds[len(bounds)-1] != s {
			bounds = append(bounds, s)
		}
	}
	return bounds
}

// splitFromBoundaries builds shards {nil,b0}, {b0,b1}, …, {b_last,nil}.
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
	return append(shards, checkpoint.KeysetShard{Lo: prev})
}

func isIntType(t string) bool {
	switch t {
	case "tinyint", "smallint", "int", "bigint":
		return true
	}
	return false
}

// Extract reads every requested table from a fresh plan.
func (s *Source) Extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts) error {
	var jobs []job
	for _, name := range opts.Resources {
		tjobs, err := s.tableJobs(ctx, sink, name, nil, opts.Limit)
		if err != nil {
			return err
		}
		jobs = append(jobs, tjobs...)
	}
	return s.runJobs(ctx, opts.Parallelism, jobs)
}

// ExtractFrom reads each table by its checkpoint: an incremental plan by its
// watermark, a keyset or incremental-backfill plan by its shards, and anything
// else from a fresh plan.
func (s *Source) ExtractFrom(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts, prev map[string]filament.Checkpoint) error {
	var incremental, rest []string
	for _, name := range opts.Resources {
		if plan, ok := checkpoint.ParseKeyset(prev[name]); ok && plan.Mode == checkpoint.ModeIncremental {
			incremental = append(incremental, name)
		} else {
			rest = append(rest, name)
		}
	}
	var jobs []job
	for _, name := range incremental {
		j, err := s.incrementalJob(ctx, sink, name, prev[name], opts.Limit)
		if err != nil {
			return err
		}
		jobs = append(jobs, j)
	}
	for _, name := range rest {
		var plan *checkpoint.KeysetCheckpoint
		if ks, ok := checkpoint.ParseKeyset(prev[name]); ok && len(ks.Cols) > 0 {
			plan = &ks
		}
		tjobs, err := s.tableJobs(ctx, sink, name, plan, opts.Limit)
		if err != nil {
			return err
		}
		jobs = append(jobs, tjobs...)
	}
	return s.runJobs(ctx, opts.Parallelism, jobs)
}

// job reads one shard on a querier the runner provides.
type job func(ctx context.Context, q querier) error

// tableJobs plans one table into its shard reads.
func (s *Source) tableJobs(ctx context.Context, sink arrowbatch.Inlet, name string, plan *checkpoint.KeysetCheckpoint, limit int) ([]job, error) {
	t, err := s.tableMeta(ctx, name)
	if err != nil {
		return nil, err
	}
	if !t.orderedKey() {
		return []job{func(ctx context.Context, q querier) error {
			r := &rowReader{schema: t.schema, limit: limit, open: builder(sink, name, 0, t.schema)}
			query := "SELECT " + top(limit) + t.selectList + " FROM " + t.qualified + " t"
			if err := readRows(ctx, q, query, nil, r); err != nil {
				return fmt.Errorf("scan %q: %w", name, err)
			}
			return nil
		}}, nil
	}
	if plan == nil {
		fresh, err := s.planKeyset(ctx, t)
		if err != nil {
			return nil, err
		}
		plan = &fresh
	}
	keys := make([]keyColumn, len(plan.Cols))
	for i, c := range plan.Cols {
		keys[i] = keyColumn{name: c, decl: typeAt(plan.Types, i)}
	}
	jobs := make([]job, len(plan.Shards))
	for i, sh := range plan.Shards {
		ks := keyShard{t: t, keys: keys, part: i, lo: sh.Lo, hi: sh.Hi, seed: sh.Key}
		jobs[i] = func(ctx context.Context, q querier) error {
			return s.readShard(ctx, sink, q, ks, limit)
		}
	}
	return jobs, nil
}

// readShard streams one shard from just after its cursor (or its lower bound)
// to its upper bound in key order.
func (s *Source) readShard(ctx context.Context, sink arrowbatch.Inlet, q querier, sh keyShard, limit int) error {
	names := make([]string, len(sh.keys))
	for i, k := range sh.keys {
		names[i] = k.name
	}
	r := &rowReader{
		schema: sh.t.schema, limit: limit, keyNames: names,
		open: builder(sink, sh.t.name, sh.part, sh.t.schema),
	}
	where, args := keysetWhere(sh)
	query := fmt.Sprintf("SELECT %s%s FROM %s t%s ORDER BY %s OPTION (MAXDOP 1)",
		top(limit), sh.t.selectList, sh.t.qualified, where, keyOrder(sh.keys, ""))
	if err := readRows(ctx, q, query, args, r); err != nil {
		return fmt.Errorf("keyset %q part %d: %w", sh.t.name, sh.part, err)
	}
	return nil
}

func builder(sink arrowbatch.Inlet, name string, part int, schema rowmodel.Schema) func() (arrowbatch.RowWriter, error) {
	return func() (arrowbatch.RowWriter, error) { return sink.Builder(name, part, schema) }
}

func top(limit int) string {
	if limit <= 0 {
		return ""
	}
	return "TOP (" + strconv.Itoa(limit) + ") "
}

// keysetWhere builds a shard's predicate: after the cursor, or at or above the
// lower bound, and below the upper bound. Bounds cover a prefix of the key; the
// cursor is the whole key.
func keysetWhere(sh keyShard) (string, []any) {
	var clauses []string
	var args []any
	if len(sh.seed) == len(sh.keys) {
		clauses = append(clauses, tupleCmp(sh.keys, ">", sh.seed, &args))
	} else if len(sh.lo) > 0 {
		clauses = append(clauses, tupleCmp(sh.keys[:len(sh.lo)], ">=", sh.lo, &args))
	}
	if len(sh.hi) > 0 {
		clauses = append(clauses, tupleCmp(sh.keys[:len(sh.hi)], "<", sh.hi, &args))
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// tupleCmp renders a lexicographic comparison of key columns against values
// in the seekable expanded form, (a > x OR (a = x AND b > y)), since T-SQL
// has no row-value comparison. Each value is cast to its column's declared
// type so the comparison is typed and uses the index.
func tupleCmp(keys []keyColumn, op string, vals []string, args *[]any) string {
	strict := strings.TrimSuffix(op, "=")
	var expand func(i int) string
	expand = func(i int) string {
		col := "t." + quoteIdent(keys[i].name)
		*args = append(*args, bindValue(keys[i], vals[i]))
		ph := castParam(keys[i], len(*args))
		if i == len(keys)-1 {
			return col + " " + op + " " + ph
		}
		return "(" + col + " " + strict + " " + ph + " OR (" + col + " = " + ph + " AND " + expand(i+1) + "))"
	}
	return expand(0)
}

func castParam(k keyColumn, n int) string {
	p := "@p" + strconv.Itoa(n)
	if isBinaryDecl(k.decl) {
		return p // bound as varbinary, which compares bytewise with every binary type
	}
	if typ, collation, ok := strings.Cut(k.decl, collatePrefix); ok {
		// The key keeps its column's collation, and a conversion to non-Unicode
		// text uses its input's code page.
		return "CAST(" + p + collatePrefix + collation + " AS " + typ + ")"
	}
	return "CAST(" + p + " AS " + k.decl + ")"
}

// bindValue reverses the checkpoint text of a key value into a parameter.
func bindValue(k keyColumn, text string) any {
	switch {
	case isIntType(k.decl):
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n
		}
	case isBinaryDecl(k.decl):
		if encoded, ok := strings.CutPrefix(text, binaryCheckpointPrefix); ok {
			if b, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
				return b
			}
		}
	}
	return text
}

func isBinaryDecl(decl string) bool {
	return strings.HasPrefix(decl, "binary") || strings.HasPrefix(decl, "varbinary") || isRowversion(decl)
}

// isRowversion reports a rowversion column, which the catalog names timestamp.
func isRowversion(decl string) bool { return decl == "timestamp" || decl == "rowversion" }

// keyValue selects a key column in the form its checkpoint text takes, as
// the row read selects it. A hierarchyid travels as its path, such as /1/2/,
// which casts back to the same hierarchyid.
func keyValue(decl, ref string) string {
	if decl == "hierarchyid" {
		return ref + ".ToString()"
	}
	return ref
}

func keyOrder(keys []keyColumn, dir string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = "t." + quoteIdent(k.name) + dir
	}
	return strings.Join(parts, ", ")
}

func typeAt(types []string, i int) string {
	if i < len(types) {
		return types[i]
	}
	return "nvarchar(4000)"
}

// runJobs runs jobs with at most parallelism at once, each in its own read
// transaction. The first failure cancels the rest.
func (s *Source) runJobs(ctx context.Context, parallelism int, jobs []job) error {
	if parallelism <= 1 || len(jobs) <= 1 {
		for _, j := range jobs {
			if err := s.inReadTx(ctx, j); err != nil {
				return err
			}
		}
		return nil
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	fail := func(err error) {
		once.Do(func() {
			first = err
			cancel(err)
		})
	}
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				// A job that never ran fails the read, so a cancelled caller
				// does not see a partial read as a success.
				fail(context.Cause(ctx))
				return
			}
			if err := s.inReadTx(ctx, j); err != nil {
				fail(err)
			}
		}()
	}
	wg.Wait()
	return first
}

// inReadTx runs j in a transaction at the source's isolation level. Under
// SNAPSHOT the shard reads one point in time however long it streams, without
// taking shared locks; each shard has its own point in time, since SQL Server
// cannot share a snapshot between sessions.
func (s *Source) inReadTx(ctx context.Context, j job) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: s.isolation})
	if err != nil {
		return fmt.Errorf("begin read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	return j(ctx, tx)
}
