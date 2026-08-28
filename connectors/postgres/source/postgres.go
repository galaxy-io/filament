// Package postgres implements the filament.Source interface as a full-snapshot
// (ModeFull) reader for PostgreSQL.
//
// Each requested resource is a table name. A table's heap is sliced into one or
// more shards by physical ctid block range, and each shard is read window by window:
// WHERE ctid >= '(lo,0)' AND ctid < '(hi,0)' over a fixed span of blocks at a time.
// Reading by ctid (not the primary key) means one reader handles primary-key,
// composite-key, and keyless tables identically. On PostgreSQL 14+ each bounded
// window is a TID Range Scan:
// sequential heap I/O, no index needed. Both bounds matter — an open-ended ctid > x
// re-scans to end-of-table on every page, so every window is bounded on both sides.
//
// Large tables split into several shards so a single big table no longer pins one
// worker while the rest sit idle (see planShards / shard_pages). When more than one
// shard runs concurrently the reads share an exported snapshot (pg_export_snapshot),
// so every shard sees one consistent point-in-time even under concurrent writes —
// the same mechanism pg_dump -j uses.
//
// Rows are read as typed columns over pgx's binary protocol and appended straight
// into the pipeline's row writers — see columns.go — so neither the source
// database nor this process ever renders a row as text.
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
	"github.com/galaxy-io/filament/arrowbatch"
	pgconnection "github.com/galaxy-io/filament/connectors/postgres/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
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

	defaultPublication = "filament"
	defaultSlotName    = "filament"

	// The replication config values: query-based reads or the WAL stream.
	replicationStandard = "standard"
	replicationCDC      = "cdc"
)

// Source reads tables from a PostgreSQL database as a full snapshot. One instance
// is created per run: Configure opens the pool, Extract pages the tables, Teardown
// closes the pool.
type Source struct {
	pool              *pgxpool.Pool
	dsn               string
	schema            string
	pageSize          int
	shardPages        int
	readMode          string // "", "keyset" (key-ordered), "bitmap" (unordered sub-ranges), "auto" (probe)
	cursorColumns     map[string]string
	cursorLookbacks   map[string]int
	publication       string
	slotName          string
	managePublication bool
}

// New returns an unconfigured source.
func New() *Source {
	return &Source{
		schema: defaultSchema, pageSize: defaultPageSize, shardPages: defaultShardPages,
		publication: defaultPublication, slotName: defaultSlotName, managePublication: true,
	}
}

var (
	_ filament.Source               = (*Source)(nil)
	_ filament.Discoverable         = (*Source)(nil)
	_ filament.LiveValidatable      = (*Source)(nil)
	_ filament.SchemaProvider       = (*Source)(nil)
	_ filament.Resumable            = (*Source)(nil)
	_ filament.ResumePlanner        = (*Source)(nil)
	_ filament.IncrementalPlanner   = (*Source)(nil)
	_ filament.CursorColumnProvider = (*Source)(nil)
	_ filament.ChangeSource         = (*Source)(nil)
)

// Spec describes the source's config fields, modes, and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:         "postgres",
		DisplayName:  "PostgreSQL",
		Description:  "Popular open-source relational database management system known for reliability and advanced features.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-postgres-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-postgres-light.svg",
		Version:      "2",
		Modes:        []filament.ReadMode{filament.ModeFull, filament.ModeIncremental, filament.ModeCDC},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionFullReplace,
			filament.IngestionFullUpsert,
			filament.IngestionFullAppend,
			filament.IngestionIncrementalAppend,
			filament.IngestionIncrementalUpsert,
			filament.IngestionCDCAppend,
			filament.IngestionCDCMerge,
		),
		Config: filament.ConfigSchema{Fields: append(pgconnection.Fields(), []filament.ConfigField{
			{Name: "replication", Type: filament.FieldEnum, Default: replicationStandard, Enum: []filament.EnumOption{
				{Value: replicationStandard, Label: "Standard"},
				{Value: replicationCDC, Label: "Change Data Capture (CDC)"},
			}, Scope: filament.ScopeConnection, Help: "Standard reads tables with queries; CDC streams changes from the write-ahead log"},
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Schema to read tables from"},
			{Name: "page_size", Type: filament.FieldInt, Default: defaultPageSize, Scope: filament.ScopePipeline, Help: "Rows to target per read page"},
			{Name: "shard_pages", Type: filament.FieldInt, Default: defaultShardPages, Scope: filament.ScopePipeline, Help: "Heap blocks per shard; 0 disables sharding"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "scan_strategy", Type: filament.FieldEnum, Default: "keyset", Enum: []filament.EnumOption{
				{Value: "auto", Label: "Auto"},
				{Value: "keyset", Label: "Keyset"},
				{Value: "bitmap", Label: "Bitmap"},
				{Value: "ctid", Label: "CTID + xmin"},
			}, Scope: filament.ScopePipeline, Help: "Read strategy"},
			{
				Name: "publication", Type: filament.FieldString, Default: defaultPublication, Scope: filament.ScopeConnection,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Logical replication publication used by CDC",
			},
			{
				Name: "manage_publication", Type: filament.FieldBool, Default: true, Scope: filament.ScopeConnection,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Create the CDC publication and add selected tables when needed",
			},
			{Name: "slot_name", Type: filament.FieldString, Default: defaultSlotName, Scope: filament.ScopePipeline, Help: "Persistent logical replication slot; use a unique slot per CDC pipeline"},
		}...)},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}

// Replication reports the mode the connection's config selects.
func (s *Source) Replication(cfg filament.Config) filament.ReplicationMode {
	if cfg.String("replication") == replicationCDC {
		return filament.ReplicationCDC
	}
	return filament.ReplicationStandard
}

// Validate rejects an invalid URL or incomplete individual connection fields.
func (s *Source) Validate(cfg filament.Config) error {
	if _, err := pgconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("postgres source: connection config: %w", err)
	}
	for field, fallback := range map[string]string{"publication": defaultPublication, "slot_name": defaultSlotName} {
		value := cfg.String(field)
		if value == "" {
			value = fallback
		}
		if !validReplicationName(value) {
			return fmt.Errorf("postgres source: %s %q must be a lowercase PostgreSQL identifier", field, value)
		}
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the database.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := pgconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("postgres source: connection config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, resolved.Pool)
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
	if v := cfg.String("scan_strategy"); v != "" {
		s.readMode = v
	}
	if v := cfg.String("publication"); v != "" {
		s.publication = v
	}
	if v := cfg.String("slot_name"); v != "" {
		s.slotName = v
	}
	if cfg.Has("manage_publication") {
		s.managePublication = cfg.Bool("manage_publication")
	}
	resolved, err := pgconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("postgres source: connection config: %w", err)
	}
	s.dsn = resolved.DSN
	poolCfg := resolved.Pool
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
func (s *Source) Extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts) error {
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
// qualified and dec are precomputed so the window loop is pure string formatting.
type shard struct {
	table        string      // raw table name, used as the record resource
	part         int         // stable per-table shard identity for the pipeline builder
	qualified    string      // sanitized "schema"."table" for the FROM clause
	dec          *rowDecoder // row decoder, shared read-only by the table's shards
	loBlock      int         // inclusive first heap block
	hiBlock      int         // exclusive last heap block
	windowBlocks int         // blocks read per query, sized to ≈ page_size rows
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
		dec, err := s.decoderFor(ctx, table, pkNames(pks))
		if err != nil {
			return nil, err
		}

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
			shards = append(shards, shard{
				table: table, part: i, qualified: qualified, dec: dec,
				loBlock: lo, hiBlock: hi, windowBlocks: window,
			})
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

// extractShard reads one shard's block windows into a fresh writer for the table.
// Each window is a half-open block range bound on both ends, so the query is a
// bounded TID range scan reading only that window — never the rest of the table.
// limit > 0 stops the shard after that many rows (best-effort per shard when
// split).
func (s *Source) extractShard(ctx context.Context, sink arrowbatch.Inlet, q querier, sh shard, limit int) error {
	w, err := sink.Builder(sh.table, sh.part, sh.dec.schema)
	if err != nil {
		return err
	}
	_, err = s.readBlocks(ctx, w, q, sh, "", nil, rowmodel.Meta{}, limit)
	return err
}

// readBlocks appends the shard's block windows into w, one bounded TID range scan
// per window. filter is an extra AND clause on t (or empty) with its args after
// the two ctid bounds. Returns the rows appended; limit > 0 stops after that many.
func (s *Source) readBlocks(ctx context.Context, w arrowbatch.RowWriter, q querier, sh shard, filter string, extra []any, meta rowmodel.Meta, limit int) (int, error) {
	sql := ctidWindowSQL(sh, filter)
	emitted := 0
	for b := sh.loBlock; b < sh.hiBlock; b += sh.windowBlocks {
		hi := min(b+sh.windowBlocks, sh.hiBlock)
		args := append([]any{fmt.Sprintf("(%d,0)", b), fmt.Sprintf("(%d,0)", hi)}, extra...)
		n, _, err := s.appendPage(ctx, q, sql, w, sh.dec, meta, nil, remaining(limit, emitted), args...)
		if err != nil {
			return emitted, err
		}
		emitted += n
		if limit > 0 && emitted >= limit {
			return emitted, nil
		}
	}
	return emitted, nil
}

// ctidWindowSQL builds one shard's block-window query, with filter (an extra AND
// clause on t, or empty) appended to the ctid range.
func ctidWindowSQL(sh shard, filter string) string {
	return fmt.Sprintf(
		"SELECT %[1]s FROM %[2]s t WHERE t.ctid >= $1::tid AND t.ctid < $2::tid%[3]s",
		sh.dec.selectList, sh.qualified, filter)
}

// binaryResults asks the server for binary wire format on every result column.
// Columns projected ::text are byte-identical in either format.
var binaryResults = pgx.QueryResultFormats{pgx.BinaryFormatCode}

// appendPage runs one query in binary result format and appends every row into
// w. keyIdx names the columns whose text forms become each row's RowMeta.Key
// (the keyset or incremental cursor); a nil keyIdx ends rows with meta as given.
// limit > 0 stops after that many rows. Returns the rows appended and the last
// row's key.
func (s *Source) appendPage(ctx context.Context, q querier, sql string, w arrowbatch.RowWriter, dec *rowDecoder, meta rowmodel.Meta, keyIdx []int, limit int, args ...any) (int, []string, error) {
	rows, err := q.Query(ctx, sql, append([]any{binaryResults}, args...)...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	n := 0
	var last []string
	for rows.Next() {
		raw := rows.RawValues()
		if keyIdx != nil {
			if meta.Key, err = dec.texts(raw, keyIdx); err != nil {
				return n, last, err
			}
		}
		if err := dec.appendRow(w, raw); err != nil {
			return n, last, fmt.Errorf("read row: %w", err)
		}
		if err := w.EndRow(meta); err != nil {
			return n, last, err
		}
		last = meta.Key
		n++
		if limit > 0 && n >= limit {
			break
		}
	}
	return n, last, rows.Err()
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
func (s *Source) extractShardSnapshot(ctx context.Context, sink arrowbatch.Inlet, snap *snapshot, sh shard, limit int) error {
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
	err = s.extractShard(ctx, sink, tx, sh, limit)
	return err
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
// An empty result means no primary key.
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
func (s *Source) Schema(ctx context.Context, resource string) (rowmodel.Schema, error) {
	cols, err := s.columns(ctx, s.schema, resource)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	pks, err := s.lookupPrimaryKey(ctx, s.schema, resource)
	if err != nil {
		return rowmodel.Schema{}, fmt.Errorf("lookup pk: %w", err)
	}
	return schemaOf(resource, cols, pkNames(pks)), nil
}

// Teardown closes the pool.
func (s *Source) Teardown(context.Context) error {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
	return nil
}
