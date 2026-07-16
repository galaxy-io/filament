// Package postgres implements the filament.Source interface as a full-snapshot
// (ModeFull) reader for PostgreSQL.
//
// Each requested resource is a table name. A table's heap is sliced into one or
// more shards by physical ctid block range, and each shard is read window by window:
// WHERE ctid >= '(lo,0)' AND ctid < '(hi,0)' over a fixed span of blocks at a time.
// Reading by ctid (not the primary key) means one reader handles primary-key,
// composite-key, and keyless tables identically — the primary key is consulted only
// to derive the record id. On PostgreSQL 14+ each bounded window is a TID Range Scan:
// sequential heap I/O, no index needed. Both bounds matter — an open-ended ctid > x
// re-scans to end-of-table on every page, so every window is bounded on both sides.
//
// Large tables split into several shards so a single big table no longer pins one
// worker while the rest sit idle (see planShards / shard_pages). When more than one
// shard runs concurrently the reads share an exported snapshot (pg_export_snapshot),
// so every shard sees one consistent point-in-time even under concurrent writes —
// the same mechanism pg_dump -j uses.
//
// Rows are encoded server-side with to_jsonb(t)::text and scanned straight into
// Record.Data, avoiding any client-side marshaling. The record id is pulled from
// that same jsonb (data->>'<pk>' == id by construction); a keyless table uses its
// ctid as the synthetic id.
package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
)

const (
	// defaultPageSize bounds rows held in memory per keyset page.
	defaultPageSize = 1000

	// defaultSchema is the namespace resources resolve under when "schema" is unset.
	defaultSchema = "public"

	// defaultShardPages is the target heap-block count per shard (8 KiB blocks →
	// ~128 MiB). A table at or below this size reads as a single shard; bigger tables
	// split so they no longer bottleneck one worker. Set shard_pages=0 to disable
	// intra-table sharding (back to one shard per table).
	defaultShardPages = 16384

	// maxShardsPerTable caps the fan-out so a multi-terabyte table cannot spawn an
	// unbounded number of shards.
	maxShardsPerTable = 64

	// defaultRowsPerBlock is the rows/block estimate used when block 0 is empty (so
	// the live sample is unusable). ~60 narrow rows per 8 KiB block is a reasonable
	// middle.
	defaultRowsPerBlock = 60

	// maxWindowBlocks caps a read window so a table of unexpectedly dense (tiny) rows
	// cannot make one window pull an unbounded number of rows into memory.
	maxWindowBlocks = 256
)

// Source reads tables from a PostgreSQL database as a full snapshot. One instance
// is created per run: Configure opens the pool, Extract pages the tables, Teardown
// closes the pool.
type Source struct {
	pool       *pgxpool.Pool
	schema     string
	pageSize   int
	shardPages int
	readMode   string // "", "keyset" (key-ordered), "bitmap" (unordered sub-ranges), "auto" (probe)
}

// New returns an unconfigured source.
func New() *Source {
	return &Source{schema: defaultSchema, pageSize: defaultPageSize, shardPages: defaultShardPages}
}

var (
	_ filament.Source          = (*Source)(nil)
	_ filament.Discoverable    = (*Source)(nil)
	_ filament.LiveValidatable = (*Source)(nil)
	_ filament.SchemaProvider  = (*Source)(nil)
	_ filament.Resumable       = (*Source)(nil)
	_ filament.ResumePlanner   = (*Source)(nil)
)

// Spec describes the source's config fields, modes, and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:        "postgres",
		DisplayName: "PostgreSQL",
		Version:     "1",
		Modes:       []filament.ReplicationMode{filament.ModeFull},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionSnapshotReplace,
			filament.IngestionSnapshotUpsert,
			filament.IngestionAppend,
		),
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "dsn", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "PostgreSQL connection string"},
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Schema to read tables from"},
			{Name: "page_size", Type: filament.FieldInt, Default: defaultPageSize, Scope: filament.ScopePipeline, Help: "Rows to target per read page"},
			{Name: "shard_pages", Type: filament.FieldInt, Default: defaultShardPages, Scope: filament.ScopePipeline, Help: "Heap blocks per shard; 0 disables sharding"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "read_mode", Type: filament.FieldEnum, Enum: []string{"auto", "keyset", "bitmap"}, Scope: filament.ScopePipeline, Help: "Read strategy"},
		}},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}

// Validate rejects a config missing the connection string.
func (s *Source) Validate(cfg filament.Config) error {
	if cfg.String("dsn") == "" {
		return fmt.Errorf("postgres source: dsn is required")
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the database.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.Secret("dsn"))
	if err != nil {
		return fmt.Errorf("postgres source: parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("postgres source: open pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres source: ping: %w", err)
	}
	return nil
}

// Configure reads dsn/schema/page_size/shard_pages/max_conns and opens a connection
// pool. Set max_conns to at least the run's parallelism + 1 (the snapshot
// coordinator holds one connection) so concurrent shard reads each get a connection
// instead of queueing; unset keeps pgx's default (max(4, NumCPU)).
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	if v := cfg.String("schema"); v != "" {
		s.schema = v
	}
	if cfg.Has("page_size") {
		if n := cfg.Int("page_size"); n > 0 {
			s.pageSize = n
		}
	}
	if cfg.Has("shard_pages") {
		// 0 is a valid value here (disable sharding), so honor it as-is.
		s.shardPages = cfg.Int("shard_pages")
	}
	if v := cfg.String("read_mode"); v != "" {
		s.readMode = v
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.Secret("dsn"))
	if err != nil {
		return fmt.Errorf("postgres source: parse dsn: %w", err)
	}
	if cfg.Has("max_conns") {
		if n := cfg.Int("max_conns"); n > 0 && n <= math.MaxInt32 {
			poolCfg.MaxConns = int32(n)
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("postgres source: open pool: %w", err)
	}
	s.pool = pool
	return nil
}

// Discover lists tables in the configured schema with their primary keys and row estimates.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.pool == nil {
		return filament.DiscoverResult{}, fmt.Errorf("postgres source: discover before configure")
	}
	const q = `
SELECT
	c.relname,
	COALESCE(string_agg(a.attname, ',' ORDER BY k.ord), '') AS primary_key,
	GREATEST(c.reltuples::bigint, 0) AS estimated_rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_index i ON i.indrelid = c.oid AND i.indisprimary
LEFT JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum
WHERE n.nspname = $1
  AND c.relkind IN ('r', 'p')
GROUP BY c.relname, c.reltuples
ORDER BY c.relname`
	rows, err := s.pool.Query(ctx, q, s.schema)
	if err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("postgres source: discover tables: %w", err)
	}
	defer rows.Close()

	var resources []filament.Resource
	for rows.Next() {
		var name, pkCSV string
		var estimated int64
		if err := rows.Scan(&name, &pkCSV, &estimated); err != nil {
			return filament.DiscoverResult{}, fmt.Errorf("postgres source: scan table: %w", err)
		}
		var pk []string
		if pkCSV != "" {
			pk = strings.Split(pkCSV, ",")
		}
		resources = append(resources, filament.Resource{
			Name:       name,
			Selectable: true,
			PrimaryKey: pk,
			Estimated:  estimated,
		})
	}
	if err := rows.Err(); err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("postgres source: discover rows: %w", err)
	}
	return filament.DiscoverResult{Resources: resources}, nil
}

// Extract plans every requested table into shards and pages them out through the
// sink. With Parallelism > 1 the shards are read concurrently (bounded by
// Parallelism) into the shared sink under one exported snapshot; paging within a
// shard stays sequential. The first shard error cancels the rest and is returned.
func (s *Source) Extract(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
	shards, err := s.planShards(ctx, opts.Resources, opts.Parallelism)
	if err != nil {
		return err
	}

	// Sequential path: one worker, no cross-shard concurrency, so no snapshot to
	// coordinate — read each shard in turn straight off the pool.
	if opts.Parallelism <= 1 || len(shards) <= 1 {
		for _, sh := range shards {
			if err := s.extractShard(ctx, sink, s.pool, sh, opts.Limit); err != nil {
				return fmt.Errorf("extract %q: %w", sh.table, err)
			}
		}
		return nil
	}

	// Concurrent path: pin one consistent snapshot for every shard, then fan out.
	snap, err := s.openSnapshot(ctx)
	if err != nil {
		return err
	}
	defer snap.close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, opts.Parallelism)
	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for _, sh := range shards {
		wg.Add(1)
		go func(sh shard) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return // a sibling already failed; stop launching work
			}
			if err := s.extractShardSnapshot(ctx, sink, snap, sh, opts.Limit); err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("extract %q: %w", sh.table, err)
					cancel() // stop the other readers
				})
			}
		}(sh)
	}
	wg.Wait()
	return firstErr
}

// shard is one independently-readable slice of a table's heap: the half-open heap
// block range [loBlock, hiBlock). The reader walks it in fixed windows of windowBlocks
// blocks, so every query is a bounded TID range scan (no whole-table scan, no sort).
// qualified and idExpr are precomputed so the window loop is pure string formatting.
type shard struct {
	table        string // raw table name, used as the record resource
	qualified    string // sanitized "schema"."table" for the FROM clause
	idExpr       string // SQL projecting the record id from j (jsonb) / c (ctid)
	loBlock      int    // inclusive first heap block
	hiBlock      int    // exclusive last heap block
	windowBlocks int    // blocks read per query, sized to ≈ page_size rows
}

// planShards turns each requested table into one or more shards. A table is split
// only when sharding is enabled, the run is parallel, and the table is larger than
// shard_pages; otherwise it is a single shard covering the whole heap. Every shard
// is bounded by the table's current block count — an unbounded ctid scan re-reads to
// end-of-table on each page, so it is avoided.
func (s *Source) planShards(ctx context.Context, tables []string, parallelism int) ([]shard, error) {
	var shards []shard
	for _, table := range tables {
		pks, err := s.lookupPrimaryKey(ctx, s.schema, table)
		if err != nil {
			return nil, fmt.Errorf("lookup pk %q: %w", table, err)
		}
		qualified := pgx.Identifier{s.schema, table}.Sanitize()
		idExpr := idExprFor(pks)

		pages, err := s.lookupPages(ctx, qualified)
		if err != nil {
			return nil, fmt.Errorf("lookup size %q: %w", table, err)
		}
		window, err := s.windowBlocks(ctx, qualified)
		if err != nil {
			return nil, fmt.Errorf("size window %q: %w", table, err)
		}
		k := shardCount(pages, parallelism, s.shardPages)
		for i := range k {
			lo := i * pages / k
			hi := (i + 1) * pages / k
			if i == k-1 {
				hi = pages // exact end; rows live in blocks [0, pages)
			}
			shards = append(shards, shard{table, qualified, idExpr, lo, hi, window})
		}
	}
	return shards, nil
}

// shardCount decides how many shards a table of `pages` heap blocks becomes.
func shardCount(pages, parallelism, shardPages int) int {
	if shardPages <= 0 || parallelism <= 1 || pages <= shardPages {
		return 1
	}
	return min((pages+shardPages-1)/shardPages, maxShardsPerTable)
}

// idExprFor builds the SQL that projects a record id from a shard's row. With a
// single-column key the id is data->>'<pk>'; a composite key joins the columns with
// chr(31) (a separator that cannot appear in normal keys); a keyless table falls
// back to the row's ctid (unique within the snapshot).
func idExprFor(pks []pkColumn) string {
	if len(pks) == 0 {
		return "c::text"
	}
	parts := make([]string, len(pks))
	for i, pk := range pks {
		parts[i] = "j ->> " + quoteLiteral(pk.name)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "concat_ws(chr(31), " + strings.Join(parts, ", ") + ")"
}

// quoteLiteral renders s as a single-quoted SQL string literal. Used only for
// catalog-sourced column names that must be inlined into a SELECT projection.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// pkColumn is one primary-key column: its raw name (bound into the id projection) and
// Postgres type (used to cast a text resume cursor back to the column type on the
// keyset path; ignored on the ctid path).
type pkColumn struct{ name, typ string }

// querier is the subset of pgx used by the page loop, satisfied by both the pool
// and a snapshot transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// extractShard reads one shard out through the sink using q, one ctid block window
// at a time. Each window is a half-open block range bound on both ends, so the query
// is a bounded TID range scan reading only that window — never the rest of the table.
// limit > 0 stops the shard after that many rows (best-effort per shard when split).
func (s *Source) extractShard(ctx context.Context, sink filament.RecordSink, q querier, sh shard, limit int) error {
	// Both ctid bounds are bound parameters; the scan reads blocks [lo, hi).
	sql := fmt.Sprintf(`
SELECT %[1]s AS id, j::text AS data
FROM (
	SELECT to_jsonb(t) AS j, t.ctid AS c
	FROM %[2]s t
	WHERE t.ctid >= $1::tid AND t.ctid < $2::tid
) page`, sh.idExpr, sh.qualified)

	emitted := 0
	for b := sh.loBlock; b < sh.hiBlock; b += sh.windowBlocks {
		hi := min(b+sh.windowBlocks, sh.hiBlock)
		lo := fmt.Sprintf("(%d,0)", b)
		hiTid := fmt.Sprintf("(%d,0)", hi)
		page, err := s.readWindow(ctx, q, sql, lo, hiTid, sh.table)
		if err != nil {
			return err
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
	}
	return nil
}

// readWindow runs one block window and returns its records. Rows are drained and
// closed before returning so the connection is free before records are pushed (a
// push can block on backpressure).
func (s *Source) readWindow(ctx context.Context, q querier, sql, lo, hi, table string) ([]filament.Record, error) {
	rows, err := q.Query(ctx, sql, lo, hi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]filament.Record, 0, s.pageSize)
	// Scan destinations are hoisted and reused: pgx writes a freshly allocated
	// string and []byte into id/data on every row (so each Record keeps its own
	// backing), while the locals and the dest slice are allocated once for the whole
	// window instead of per row — three fewer heap objects per row.
	var (
		id   string
		data []byte
	)
	dest := []any{&id, &data}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		out = append(out, filament.NewRecord(table, id, data))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// snapshot holds an exported-snapshot transaction open for a run's lifetime. Its
// connection stays checked out of the pool so the snapshot remains importable by
// every shard transaction; close releases both.
type snapshot struct {
	conn *pgxpool.Conn
	tx   pgx.Tx
	id   string
}

// openSnapshot begins a read-only repeatable-read transaction and exports its
// snapshot id. Shard transactions import it via SET TRANSACTION SNAPSHOT so all
// shards read one consistent point-in-time.
func (s *Source) openSnapshot(ctx context.Context) (*snapshot, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire snapshot conn: %w", err)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("begin snapshot tx: %w", err)
	}
	var id string
	if err := tx.QueryRow(ctx, "SELECT pg_export_snapshot()").Scan(&id); err != nil {
		_ = tx.Rollback(ctx)
		conn.Release()
		return nil, fmt.Errorf("export snapshot: %w", err)
	}
	return &snapshot{conn: conn, tx: tx, id: id}, nil
}

func (sn *snapshot) close(ctx context.Context) {
	if sn == nil {
		return
	}
	_ = sn.tx.Rollback(ctx)
	sn.conn.Release()
}

// extractShardSnapshot reads one shard inside its own transaction that imports the
// run's exported snapshot, then delegates to the shared page loop.
func (s *Source) extractShardSnapshot(ctx context.Context, sink filament.RecordSink, snap *snapshot, sh shard, limit int) error {
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

	// The snapshot id is server-generated; SET TRANSACTION SNAPSHOT takes no bind
	// parameter, so it is quoted and inlined.
	if _, err := tx.Exec(ctx, "SET TRANSACTION SNAPSHOT "+quoteLiteral(snap.id)); err != nil {
		return fmt.Errorf("import snapshot: %w", err)
	}
	return s.extractShard(ctx, sink, tx, sh, limit)
}

// lookupPages returns the current heap size of the table in 8 KiB blocks, derived
// from pg_relation_size so it reflects live size even for a freshly loaded table
// that has never been ANALYZEd (pg_class.relpages would still read 0). The count
// bounds every shard at the real end of the heap.
func (s *Source) lookupPages(ctx context.Context, qualified string) (int, error) {
	const q = `SELECT (pg_relation_size($1::regclass) / current_setting('block_size')::bigint)::int`
	var pages int
	if err := s.pool.QueryRow(ctx, q, qualified).Scan(&pages); err != nil {
		return 0, err
	}
	return pages, nil
}

// windowBlocks sizes the per-query block window so a window holds roughly page_size
// rows: it samples the live row count of heap block 0 and divides. Bounding the read
// by blocks (not a row LIMIT + keyset) keeps each query a tightly-bounded TID range
// scan, the cost the open-ended cursor used to blow up.
func (s *Source) windowBlocks(ctx context.Context, qualified string) (int, error) {
	// count(*) over block 0 only — a single-block TID range scan.
	q := "SELECT count(*) FROM " + qualified + " t WHERE t.ctid < '(1,0)'::tid"
	var rowsPerBlock int
	if err := s.pool.QueryRow(ctx, q).Scan(&rowsPerBlock); err != nil {
		return 0, err
	}
	if rowsPerBlock < 1 {
		rowsPerBlock = defaultRowsPerBlock
	}
	w := max(s.pageSize/rowsPerBlock, 1)
	return min(w, maxWindowBlocks), nil
}

// lookupPrimaryKey returns the primary-key columns of schema.table in key order.
// An empty result means no primary key, so the record id falls back to ctid.
func (s *Source) lookupPrimaryKey(ctx context.Context, schema, table string) ([]pkColumn, error) {
	const q = `
SELECT a.attname, format_type(a.atttypid, a.atttypmod)
FROM   pg_index i
JOIN   pg_class cl ON cl.oid = i.indrelid
JOIN   pg_namespace ns ON ns.oid = cl.relnamespace
JOIN   pg_attribute a ON a.attrelid = cl.oid AND a.attnum = ANY(i.indkey)
WHERE  ns.nspname = $1 AND cl.relname = $2 AND i.indisprimary
ORDER  BY array_position(i.indkey, a.attnum)`
	rows, err := s.pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pks []pkColumn
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		pks = append(pks, pkColumn{name: name, typ: typ})
	}
	return pks, rows.Err()
}

// Schema returns the column schema of one table: every live column in attribute
// order with its Postgres type (format_type, kept verbatim in Native for a same-engine
// round-trip and classified into a LogicalType), plus the primary key. It implements
// filament.SchemaProvider so a Schematized sink can build matching typed tables.
func (s *Source) Schema(ctx context.Context, resource string) (filament.RecordSchema, error) {
	const q = `
SELECT a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull
FROM   pg_attribute a
JOIN   pg_class cl ON cl.oid = a.attrelid
JOIN   pg_namespace ns ON ns.oid = cl.relnamespace
WHERE  ns.nspname = $1 AND cl.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
ORDER  BY a.attnum`
	rows, err := s.pool.Query(ctx, q, s.schema, resource)
	if err != nil {
		return filament.RecordSchema{}, err
	}
	defer rows.Close()
	var fields []filament.SchemaField
	for rows.Next() {
		var name, native string
		var notNull bool
		if err := rows.Scan(&name, &native, &notNull); err != nil {
			return filament.RecordSchema{}, fmt.Errorf("scan column: %w", err)
		}
		fields = append(fields, filament.SchemaField{
			Name:     name,
			Nullable: !notNull,
			Logical:  pgTypeToLogical(native),
			Native:   native,
		})
	}
	if err := rows.Err(); err != nil {
		return filament.RecordSchema{}, err
	}
	if len(fields) == 0 {
		return filament.RecordSchema{}, fmt.Errorf("resource %q has no columns (missing table?)", resource)
	}
	pks, err := s.lookupPrimaryKey(ctx, s.schema, resource)
	if err != nil {
		return filament.RecordSchema{}, fmt.Errorf("lookup pk: %w", err)
	}
	key := make([]string, len(pks))
	for i, pk := range pks {
		key[i] = pk.name
	}
	return filament.RecordSchema{Resource: resource, Fields: fields, PrimaryKey: key}, nil
}

// pgTypeToLogical classifies a Postgres format_type string into a portable
// LogicalType. The exact type (including precision/scale and array element) is kept
// in SchemaField.Native, so an unknown type degrading to string is harmless for a
// same-engine (Postgres→Postgres) sink that reuses Native verbatim.
func pgTypeToLogical(native string) filament.LogicalType {
	t := strings.ToLower(strings.TrimSpace(native))
	if strings.HasSuffix(t, "[]") {
		return filament.LogicalArray
	}
	if i := strings.IndexByte(t, '('); i >= 0 { // drop a type modifier like (12,2)
		t = strings.TrimSpace(t[:i])
	}
	switch t {
	case "boolean":
		return filament.LogicalBool
	case "smallint", "int2":
		return filament.LogicalInt16
	case "integer", "int", "int4":
		return filament.LogicalInt32
	case "bigint", "int8":
		return filament.LogicalInt64
	case "real", "float4":
		return filament.LogicalFloat32
	case "double precision", "float8":
		return filament.LogicalFloat64
	case "numeric", "decimal":
		return filament.LogicalDecimal
	case "uuid":
		return filament.LogicalUUID
	case "json", "jsonb":
		return filament.LogicalJSON
	case "bytea":
		return filament.LogicalBytes
	case "date":
		return filament.LogicalDate
	case "timestamp without time zone":
		return filament.LogicalTimestamp
	case "timestamp with time zone":
		return filament.LogicalTimestampTZ
	case "time without time zone", "time with time zone":
		return filament.LogicalTime
	default:
		return filament.LogicalString
	}
}

// Teardown closes the pool.
func (s *Source) Teardown(context.Context) error {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
	return nil
}
