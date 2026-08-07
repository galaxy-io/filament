// Package mysql implements the filament.Source interface as a full-snapshot
// (ModeFull) reader for MySQL (8.0+).
//
// Each requested resource is a table name. InnoDB stores rows clustered by primary
// key, so key order is physical order: a keyset read (WHERE pk > cursor ORDER BY pk
// LIMIT page) over the clustered index is sequential I/O. That removes the need for
// a physical read tier.
//
// Large tables split into several primary-key range shards so a single big table doesnt
// pin one worker while the rest sit idle (see planKeyset / shard_pages). An
// integer leading key splits by arithmetic over min/max; any other leading key splits
// by sampled equal-count boundaries.
//
// Rows are encoded server-side with JSON_OBJECT over the table's column list and
// scanned straight into Record.Data, avoiding client-side marshaling. Binary columns
// are wrapped in TO_BASE64 which gets transported as LogicalBytes type
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/galaxy-io/filament"
)

const (
	// defaultPageSize bounds rows held in memory per keyset page.
	defaultPageSize = 1000

	// defaultShardPages is the target InnoDB page count per shard (16 KiB pages →
	// ~256 MiB). A table at or below this size reads as a single shard; bigger tables
	// split so they no longer bottleneck one worker. Set shard_pages=0 to disable
	// intra-table sharding (back to one shard per table).
	defaultShardPages = 16384

	// maxShardsPerTable caps the fan-out so a multi-terabyte table cannot spawn an
	// unbounded number of shards.
	maxShardsPerTable = 64
)

// Source reads tables from a MySQL database as a full snapshot. One instance is
// created per run: Configure opens the pool, Extract pages the tables, Teardown
// closes the pool.
type Source struct {
	db         *sql.DB
	database   string
	pageSize   int
	shardPages int

	// Replication-client identity and endpoint for the CDC path (see cdc.go),
	// captured from the DSN at Configure.
	serverID   uint32
	binlogHost string
	binlogPort uint16
	binlogUser string
	binlogPass string
}

// New returns an unconfigured source.
func New() *Source {
	return &Source{pageSize: defaultPageSize, shardPages: defaultShardPages}
}

// querier is the subset of database/sql used by the page loop, satisfied by *sql.DB,
// *sql.Conn, and *sql.Tx. PrepareContext lets the keyset loop prepare its page
// statement once per shard: database/sql otherwise turns every parameterized
// QueryContext into prepare → execute → close (three round trips per page).
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

var (
	_ filament.Source               = (*Source)(nil)
	_ filament.Discoverable         = (*Source)(nil)
	_ filament.LiveValidatable      = (*Source)(nil)
	_ filament.SchemaProvider       = (*Source)(nil)
	_ filament.CursorColumnProvider = (*Source)(nil)
	_ filament.Resumable            = (*Source)(nil)
	_ filament.ResumePlanner        = (*Source)(nil)
)

// Spec describes the source's config fields, modes, and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:         "mysql",
		DisplayName:  "MySQL",
		Description:  "Widely-used open-source relational database known for speed, reliability, and ease of use.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-mysql-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-mysql-light.svg",
		Version:      "1",
		Modes:        []filament.ReplicationMode{filament.ModeFull, filament.ModeCDC},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionSnapshotReplace,
			filament.IngestionSnapshotUpsert,
			filament.IngestionSnapshotAppend,
			filament.IngestionCDC,
		),
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "dsn", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "MySQL connection string (user:pass@tcp(host:port)/dbname)"},
			{Name: "database", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Database to read tables from (defaults to the DSN's database)"},
			{Name: "page_size", Type: filament.FieldInt, Default: defaultPageSize, Scope: filament.ScopePipeline, Help: "Rows to target per read page"},
			{Name: "shard_pages", Type: filament.FieldInt, Default: defaultShardPages, Scope: filament.ScopePipeline, Help: "InnoDB pages per shard; 0 disables sharding"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "server_id", Type: filament.FieldInt, Default: defaultServerID, Scope: filament.ScopePipeline, Help: "Replication client server_id for CDC (must be unique in the replica topology)"},
		}},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}

// Validate rejects a config missing the connection string or one whose DSN names no
// database when "database" is also unset.
func (s *Source) Validate(cfg filament.Config) error {
	if cfg.String("dsn") == "" {
		return fmt.Errorf("mysql source: dsn is required")
	}
	if cfg.String("database") == "" {
		mc, err := mysql.ParseDSN(cfg.Secret("dsn"))
		if err != nil {
			return fmt.Errorf("mysql source: parse dsn: %w", err)
		}
		if mc.DBName == "" {
			return fmt.Errorf("mysql source: dsn has no database and \"database\" is unset")
		}
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the database.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	db, err := sql.Open("mysql", cfg.Secret("dsn"))
	if err != nil {
		return fmt.Errorf("mysql source: open: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql source: ping: %w", err)
	}
	return nil
}

// Configure reads dsn/database/page_size/shard_pages/max_conns and opens a connection
// pool. Set max_conns to at least the run's parallelism so concurrent shard reads
// each get a connection instead of queueing.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	mc, err := mysql.ParseDSN(cfg.Secret("dsn"))
	if err != nil {
		return fmt.Errorf("mysql source: parse dsn: %w", err)
	}
	s.database = mc.DBName
	if v := cfg.String("database"); v != "" {
		s.database = v
		mc.DBName = v
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
	s.serverID = defaultServerID
	if cfg.Has("server_id") {
		if n := cfg.Int("server_id"); n > 0 && n <= math.MaxUint32 {
			s.serverID = uint32(n)
		}
	}
	host, port := splitHostPort(mc.Addr)
	s.binlogHost, s.binlogPort = host, port
	s.binlogUser, s.binlogPass = mc.User, mc.Passwd

	db, err := sql.Open("mysql", mc.FormatDSN())
	if err != nil {
		return fmt.Errorf("mysql source: open: %w", err)
	}
	if cfg.Has("max_conns") {
		if n := cfg.Int("max_conns"); n > 0 {
			db.SetMaxOpenConns(n)
		}
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("mysql source: ping: %w", err)
	}
	s.db = db
	return nil
}

// Discover lists tables in the configured database with their primary keys and row estimates.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.db == nil {
		return filament.DiscoverResult{}, fmt.Errorf("mysql source: discover before configure")
	}
	const q = `
SELECT
	t.TABLE_NAME,
	COALESCE((
		SELECT GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',')
		FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA = t.TABLE_SCHEMA AND k.TABLE_NAME = t.TABLE_NAME
		  AND k.CONSTRAINT_NAME = 'PRIMARY'
	), '') AS primary_key,
	COALESCE(t.TABLE_ROWS, 0) AS estimated_rows
FROM information_schema.TABLES t
WHERE t.TABLE_SCHEMA = ? AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY t.TABLE_NAME`
	rows, err := s.db.QueryContext(ctx, q, s.database)
	if err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("mysql source: discover tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var resources []filament.Resource
	for rows.Next() {
		var name, pkCSV string
		var estimated int64
		if err := rows.Scan(&name, &pkCSV, &estimated); err != nil {
			return filament.DiscoverResult{}, fmt.Errorf("mysql source: scan table: %w", err)
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
		return filament.DiscoverResult{}, fmt.Errorf("mysql source: discover rows: %w", err)
	}
	return filament.DiscoverResult{Resources: resources}, nil
}

// Extract plans every requested table into keyset shards and pages them out through
// the sink. With Parallelism > 1 the shards are read concurrently (bounded by
// Parallelism), each inside its own consistent-snapshot transaction; paging within a
// shard stays sequential. The first shard error cancels the rest and is returned.
func (s *Source) Extract(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
	var jobs []func(context.Context, querier) error
	for _, table := range opts.Resources {
		tjobs, err := s.tableJobs(ctx, sink, table, nil, opts.Limit)
		if err != nil {
			return err
		}
		jobs = append(jobs, tjobs...)
	}
	return s.runConcurrent(ctx, opts.Parallelism, jobs)
}

// tableJobs plans one table into its shard read jobs: a fresh keyset plan for a keyed
// table, or a single streaming full scan for a keyless one. prev seeds each keyset
// shard's cursor when resuming.
func (s *Source) tableJobs(ctx context.Context, sink filament.RecordSink, table string, ks *keysetPlan, limit int) ([]func(context.Context, querier) error, error) {
	cols, pks, err := s.tableMeta(ctx, table)
	if err != nil {
		return nil, err
	}
	qualified := quoteIdent(s.database) + "." + quoteIdent(table)
	jsonExpr := jsonObjectExpr(cols)

	if len(pks) == 0 {
		// Keyless: one streaming scan with a synthetic per-snapshot row-number id.
		return []func(context.Context, querier) error{func(ctx context.Context, q querier) error {
			return s.extractKeyless(ctx, sink, q, table, qualified, jsonExpr, limit)
		}}, nil
	}

	if ks == nil {
		fresh, err := s.planKeyset(ctx, table, qualified, pks)
		if err != nil {
			return nil, err
		}
		ks = &fresh
	}
	var jobs []func(context.Context, querier) error
	for _, sh := range keyShardsFrom(table, qualified, jsonExpr, pks, *ks) {
		jobs = append(jobs, func(ctx context.Context, q querier) error {
			return s.extractKeysetShard(ctx, sink, q, sh, limit)
		})
	}
	return jobs, nil
}

// extractKeyless streams a keyless table in one pass. There is no stable key to page
// or resume by, so the whole table is read in a single query; ROW_NUMBER() supplies a
// synthetic id unique within the snapshot (the Postgres reader's ctid fallback).
func (s *Source) extractKeyless(ctx context.Context, sink filament.RecordSink, q querier, table, qualified, jsonExpr string, limit int) error {
	stmt := fmt.Sprintf("SELECT CAST(ROW_NUMBER() OVER () AS CHAR) AS id, %s AS data FROM %s t", jsonExpr, qualified)
	rows, err := q.QueryContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("scan %q: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	emitted := 0
	var (
		id   string
		data []byte
	)
	for rows.Next() {
		// Scan into *[]byte clones the driver's buffer, so data is freshly
		// allocated every row — the Record can own it without another copy.
		if err := rows.Scan(&id, &data); err != nil {
			return fmt.Errorf("read row: %w", err)
		}
		if err := sink.Push(filament.NewRecord(table, id, data)); err != nil {
			return err
		}
		emitted++
		if limit > 0 && emitted >= limit {
			return nil
		}
	}
	return rows.Err()
}

// column is one live column of a table: its name and information_schema DATA_TYPE
// (the bare type keyword, e.g. "varchar", "bigint"), plus COLUMN_TYPE (the full
// declaration, e.g. "bigint unsigned", kept in SchemaField.Native).
type column struct {
	name     string
	dataType string
	fullType string
	nullable bool
	pkOrder  int // 1-based position in the primary key; 0 = not a key column
}

// tableMeta returns the table's columns in ordinal order and its primary-key columns
// in key order, from one information_schema query.
func (s *Source) tableMeta(ctx context.Context, table string) ([]column, []pkColumn, error) {
	const q = `
SELECT c.COLUMN_NAME, c.DATA_TYPE, c.COLUMN_TYPE, c.IS_NULLABLE,
	COALESCE((
		SELECT k.ORDINAL_POSITION FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA = c.TABLE_SCHEMA AND k.TABLE_NAME = c.TABLE_NAME
		  AND k.COLUMN_NAME = c.COLUMN_NAME AND k.CONSTRAINT_NAME = 'PRIMARY'
	), 0) AS pk_order
FROM information_schema.COLUMNS c
WHERE c.TABLE_SCHEMA = ? AND c.TABLE_NAME = ?
ORDER BY c.ORDINAL_POSITION`
	rows, err := s.db.QueryContext(ctx, q, s.database, table)
	if err != nil {
		return nil, nil, fmt.Errorf("columns %q: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	var cols []column
	for rows.Next() {
		var c column
		var isNullable string
		if err := rows.Scan(&c.name, &c.dataType, &c.fullType, &isNullable, &c.pkOrder); err != nil {
			return nil, nil, fmt.Errorf("scan column: %w", err)
		}
		c.nullable = isNullable == "YES"
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("resource %q has no columns (missing table?)", table)
	}
	var pks []pkColumn
	maxOrder := 0
	for _, c := range cols {
		if c.pkOrder > maxOrder {
			maxOrder = c.pkOrder
		}
	}
	for ord := 1; ord <= maxOrder; ord++ {
		for _, c := range cols {
			if c.pkOrder == ord {
				pks = append(pks, pkColumn{name: c.name, typ: c.dataType})
			}
		}
	}
	return cols, pks, nil
}

// pkColumn is one primary-key column: its raw name (bound into the id projection) and
// its information_schema DATA_TYPE (used to cast a text resume cursor back to the
// column type — see castExpr).
type pkColumn struct{ name, typ string }

// jsonObjectExpr builds the server-side row encoder: JSON_OBJECT('col', t.`col`, …).
// Binary columns are wrapped in TO_BASE64 — MySQL JSON has no binary representation,
// and an unwrapped binary value would land as an opaque driver-specific string. The
// sink pairs this with FROM_BASE64 on the way back in.
func jsonObjectExpr(cols []column) string {
	parts := make([]string, 0, len(cols)*2)
	for _, c := range cols {
		val := "t." + quoteIdent(c.name)
		if isBinaryType(c.dataType) {
			// Strip the newlines TO_BASE64 wraps at 76 chars → canonical unwrapped base64.
			val = "REPLACE(TO_BASE64(" + val + "), '\\n', '')"
		}
		parts = append(parts, quoteLiteral(c.name), val)
	}
	return "JSON_OBJECT(" + strings.Join(parts, ", ") + ")"
}

// isBinaryType reports whether a DATA_TYPE stores raw bytes.
func isBinaryType(t string) bool {
	switch strings.ToLower(t) {
	case "binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob", "bit":
		return true
	default:
		return false
	}
}

// quoteIdent renders s as a backtick-quoted MySQL identifier.
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// splitHostPort splits a go-sql-driver Addr ("host:port", port optional) for the
// replication client.
func splitHostPort(addr string) (string, uint16) {
	host, portStr, ok := strings.Cut(addr, ":")
	if !ok {
		return addr, 3306
	}
	n, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return host, 3306
	}
	return host, uint16(n)
}

// quoteLiteral renders s as a single-quoted SQL string literal. Used only for
// catalog-sourced column names inlined into a SELECT projection.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", "''") + "'"
}

// Schema returns the column schema of one table: every column in ordinal order with
// its MySQL type (COLUMN_TYPE, kept verbatim in Native for a same-engine round-trip
// and classified into a LogicalType), plus the primary key. It implements
// filament.SchemaProvider so a Schematized sink can build matching typed tables.
func (s *Source) Schema(ctx context.Context, resource string) (filament.RecordSchema, error) {
	cols, pks, err := s.tableMeta(ctx, resource)
	if err != nil {
		return filament.RecordSchema{}, err
	}
	fields := make([]filament.SchemaField, len(cols))
	for i, c := range cols {
		fields[i] = filament.SchemaField{
			Name:     c.name,
			Nullable: c.nullable,
			Logical:  mysqlTypeToLogical(c.dataType, c.fullType),
			Native:   c.fullType,
		}
	}
	key := make([]string, len(pks))
	for i, pk := range pks {
		key[i] = pk.name
	}
	return filament.RecordSchema{Resource: resource, Fields: fields, PrimaryKey: key}, nil
}

// mysqlTypeToLogical classifies an information_schema DATA_TYPE into a portable
// LogicalType. The exact type (including display width, unsigned, enum values) is
// kept in SchemaField.Native, so an unknown type degrading to string is harmless for
// a same-engine (MySQL→MySQL) sink that reuses Native verbatim.
func mysqlTypeToLogical(dataType, fullType string) filament.LogicalType {
	t := strings.ToLower(dataType)
	unsigned := strings.Contains(strings.ToLower(fullType), "unsigned")
	switch t {
	case "tinyint":
		// tinyint(1) is MySQL's boolean idiom.
		if strings.HasPrefix(strings.ToLower(fullType), "tinyint(1)") && !unsigned {
			return filament.LogicalBool
		}
		return filament.LogicalInt16
	case "smallint":
		if unsigned {
			return filament.LogicalInt32
		}
		return filament.LogicalInt16
	case "mediumint", "int", "integer":
		if unsigned {
			return filament.LogicalInt64
		}
		return filament.LogicalInt32
	case "bigint":
		if unsigned {
			return filament.LogicalDecimal // may exceed int64
		}
		return filament.LogicalInt64
	case "float":
		return filament.LogicalFloat32
	case "double", "real":
		return filament.LogicalFloat64
	case "decimal", "numeric":
		return filament.LogicalDecimal
	case "json":
		return filament.LogicalJSON
	case "binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob", "bit":
		return filament.LogicalBytes
	case "date":
		return filament.LogicalDate
	case "datetime":
		return filament.LogicalTimestamp
	case "timestamp":
		return filament.LogicalTimestampTZ
	case "time":
		return filament.LogicalTime
	default:
		return filament.LogicalString
	}
}

// Teardown closes the pool.
func (s *Source) Teardown(context.Context) error {
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	return nil
}
