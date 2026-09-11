// Package mysql implements the filament.Source interface as a full-snapshot,
// incremental-watermark, and CDC reader for MySQL (8.0+).
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
// Rows are read as the driver's text-protocol values and parsed straight into
// the pipeline's row writers by type — see types.go — so no row is ever rendered
// as JSON on either side.
package mysql

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	mysqlconnection "github.com/galaxy-io/filament/connectors/mysql/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
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
	dsn        string // driver DSN the pool was opened with; the CDC bootstrap opens its lock session from it
	database   string
	pageSize   int
	shardPages int

	// Per-resource incremental settings populated by PlanIncremental.
	cursorColumns   map[string]string
	cursorLookbacks map[string]int

	// Replication-client identity and endpoint for the CDC path (see cdc.go),
	// captured from the DSN at Configure.
	serverID     uint32
	snapshotMode filament.SnapshotMode
	binlogHost   string
	binlogPort   uint16
	binlogUser   string
	binlogPass   string
	binlogTLS    *tls.Config
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
	_ filament.IncrementalPlanner   = (*Source)(nil)
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
		Config: filament.ConfigSchema{Fields: append(mysqlconnection.Fields(), []filament.ConfigField{
			{Name: "replication", Type: filament.FieldEnum, Default: string(filament.ReplicationStandard), Enum: []filament.EnumOption{
				{Value: string(filament.ReplicationStandard), Label: "Standard"},
				{Value: string(filament.ReplicationCDC), Label: "Change Data Capture (CDC)"},
			}, Scope: filament.ScopeConnection, Help: "Standard reads tables with queries; CDC streams changes from the binary log"},
			{Name: "database", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Database to read tables from (defaults to the DSN's database)"},
			{Name: "page_size", Type: filament.FieldInt, Default: defaultPageSize, Scope: filament.ScopePipeline, Help: "Rows to target per read page"},
			{Name: "shard_pages", Type: filament.FieldInt, Default: defaultShardPages, Scope: filament.ScopePipeline, Help: "InnoDB pages per shard; 0 disables sharding"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "server_id", Type: filament.FieldInt, Default: defaultServerID, Scope: filament.ScopePipeline, Help: "Replication client server_id for CDC (must be unique in the replica topology)"},
			{Name: "snapshot_mode", Type: filament.FieldEnum, Default: string(filament.SnapshotInitial), Enum: []filament.EnumOption{
				{Value: string(filament.SnapshotInitial), Label: "Initial"},
				{Value: string(filament.SnapshotNone), Label: "None"},
			}, Scope: filament.ScopePipeline, VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{string(filament.ReplicationCDC)}}, Help: "Initial reads a table in full before streaming its changes; None streams from the current position only"},
		}...)},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}

// Replication reports the mode the connection's config selects.
func (s *Source) Replication(cfg filament.Config) filament.ReplicationMode {
	if cfg.String("replication") == string(filament.ReplicationCDC) {
		return filament.ReplicationCDC
	}
	return filament.ReplicationStandard
}

// Validate rejects an invalid DSN or incomplete individual connection fields.
func (s *Source) Validate(cfg filament.Config) error {
	mc, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql source: connection config: %w", err)
	}
	if cfg.String("database") == "" && mc.DBName == "" {
		return fmt.Errorf("mysql source: connection has no database and \"database\" is unset")
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the database.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	mc, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql source: connection config: %w", err)
	}
	if database := cfg.String("database"); database != "" {
		mc.DBName = database
	}
	db, err := sql.Open("mysql", mc.FormatDSN())
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
	mc, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql source: connection config: %w", err)
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
	s.snapshotMode = filament.SnapshotInitial
	if v := cfg.String("snapshot_mode"); v != "" {
		s.snapshotMode = filament.SnapshotMode(v)
	}
	if mc.Net == "tcp" {
		host, port, err := splitHostPort(mc.Addr)
		if err != nil {
			return fmt.Errorf("mysql source: replication address: %w", err)
		}
		s.binlogHost, s.binlogPort = host, port
	} else if s.Replication(cfg) == filament.ReplicationCDC {
		return fmt.Errorf("mysql source: CDC requires a TCP connection, got network %q", mc.Net)
	}
	s.binlogUser, s.binlogPass = mc.User, mc.Passwd
	s.binlogTLS = nil
	if mc.TLS != nil {
		s.binlogTLS = mc.TLS.Clone()
	}

	// Timestamp columns are read as text in the session zone; pin it so they
	// decode as UTC, the same instant the binlog reports.
	if mc.Params == nil {
		mc.Params = map[string]string{}
	}
	mc.Params["time_zone"] = "'+00:00'"
	// The decoder intentionally consumes MySQL's text protocol so DATE,
	// DATETIME, and TIMESTAMP all share the same zero-value and microsecond
	// handling. A user-supplied parseTime=true DSN would otherwise make the
	// driver re-render those values as RFC3339 before scanning into RawBytes.
	mc.ParseTime = false

	s.dsn = mc.FormatDSN()
	db, err := sql.Open("mysql", s.dsn)
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
func (s *Source) Extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts) error {
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
func (s *Source) tableJobs(ctx context.Context, sink arrowbatch.Inlet, table string, ks *keysetPlan, limit int) ([]func(context.Context, querier) error, error) {
	dec, err := s.decoderFor(ctx, table)
	if err != nil {
		return nil, err
	}
	qualified := quoteIdent(s.database) + "." + quoteIdent(table)

	if len(dec.pks) == 0 {
		// Keyless: one streaming scan.
		return []func(context.Context, querier) error{func(ctx context.Context, q querier) error {
			w, err := sink.Builder(table, 0, dec.schema)
			if err != nil {
				return err
			}
			return s.extractKeyless(ctx, w, q, table, qualified, dec, limit)
		}}, nil
	}

	if ks == nil {
		fresh, err := s.planKeyset(ctx, table, qualified, dec.pks)
		if err != nil {
			return nil, err
		}
		ks = &fresh
	}
	shards, err := keyShardsFrom(table, qualified, dec, *ks)
	if err != nil {
		return nil, err
	}
	var jobs []func(context.Context, querier) error
	for _, sh := range shards {
		jobs = append(jobs, func(ctx context.Context, q querier) error {
			w, err := sink.Builder(sh.table, sh.part, sh.dec.schema)
			if err != nil {
				return err
			}
			return s.extractKeysetShard(ctx, w, q, sh, limit)
		})
	}
	return jobs, nil
}

// extractKeyless streams a keyless table in one pass into w. There is no stable key
// to page or resume by, so the whole table is read in a single query.
func (s *Source) extractKeyless(ctx context.Context, w arrowbatch.RowWriter, q querier, table, qualified string, dec *rowDecoder, limit int) error {
	rows, err := q.QueryContext(ctx, "SELECT "+dec.selectList+" FROM "+qualified+" t")
	if err != nil {
		return fmt.Errorf("scan %q: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	_, _, err = dec.appendRows(rows, w, nil, nil, limit)
	return err
}

// rowDecoder appends one scanned row's text-protocol values into a RowWriter.
// Built once per table, shared read-only by its shards.
type rowDecoder struct {
	schema     rowmodel.Schema
	selectList string
	types      []mysqlType
	pks        []pkColumn
}

// decoderFor builds table's row decoder from information_schema.
func (s *Source) decoderFor(ctx context.Context, table string) (*rowDecoder, error) {
	cols, pks, err := s.tableMeta(ctx, table)
	if err != nil {
		return nil, err
	}
	return newRowDecoder(table, cols, pks), nil
}

// newRowDecoder classifies table's columns once: the RecordSchema it reports and
// the parser for each column.
func newRowDecoder(table string, cols []column, pks []pkColumn) *rowDecoder {
	d := &rowDecoder{
		schema: rowmodel.Schema{Resource: table, PrimaryKey: pkNames(pks), Engine: engine},
		types:  make([]mysqlType, len(cols)),
		pks:    pks,
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		t := typeFor(c.dataType, c.fullType)
		parts[i], d.types[i] = "t."+quoteIdent(c.name), t
		d.schema.Fields = append(d.schema.Fields, rowmodel.Field{
			Name: c.name, Nullable: c.nullable, Logical: t.logical, Native: c.fullType, Precision: t.precision, Scale: t.scale,
		})
	}
	d.selectList = strings.Join(parts, ", ")
	return d
}

// indexOf returns the positions of the named columns; a name the live table
// lacks is an error, since a key missing from the cursor would page forever.
func (d *rowDecoder) indexOf(names []string) ([]int, error) {
	idx := make([]int, len(names))
	for n, name := range names {
		idx[n] = slices.IndexFunc(d.schema.Fields, func(f rowmodel.Field) bool { return f.Name == name })
		if idx[n] < 0 {
			return nil, fmt.Errorf("column %q not in table %q", name, d.schema.Resource)
		}
	}
	return idx, nil
}

// appendRows drains a result set into w. Every value arrives as text (the
// driver's text protocol, or its rendering of a prepared statement's typed
// values); binary columns arrive as their raw bytes. keyIdx/keyCols name and type
// the columns encoded into each row's RowMeta.Key (nil for a keyless read).
// limit > 0 stops after that many rows. Returns the rows appended and last key.
func (d *rowDecoder) appendRows(rows *sql.Rows, w arrowbatch.RowWriter, keyIdx []int, keyCols []pkColumn, limit int) (int, []string, error) {
	raw := make([]sql.RawBytes, len(d.types))
	dest := make([]any, len(raw))
	for i := range raw {
		dest[i] = &raw[i]
	}
	n := 0
	var last []string
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return n, last, fmt.Errorf("scan: %w", err)
		}
		for i, t := range d.types {
			if raw[i] == nil {
				w.Null()
				continue
			}
			if err := t.parse(w, raw[i]); err != nil {
				return n, last, fmt.Errorf("column %q: %w", d.schema.Fields[i].Name, err)
			}
		}
		var meta rowmodel.Meta
		if keyIdx != nil {
			if len(keyIdx) != len(keyCols) {
				return n, last, fmt.Errorf("checkpoint key has %d columns but %d types", len(keyIdx), len(keyCols))
			}
			meta.Key = make([]string, len(keyIdx))
			for k, i := range keyIdx {
				meta.Key[k] = keyCols[k].checkpointValue(raw[i])
			}
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
				// Keep COLUMN_TYPE rather than bare DATA_TYPE: unsigned integer
				// cursors must bind as UNSIGNED, and binary details are part of the
				// durable checkpoint's comparison contract.
				pks = append(pks, pkColumn{name: c.name, typ: c.fullType})
			}
		}
	}
	return cols, pks, nil
}

// pkColumn is one primary-key column: its raw name and information_schema
// COLUMN_TYPE, including qualifiers such as UNSIGNED and binary width. The type
// controls durable checkpoint encoding and SQL resume binding.
type pkColumn struct{ name, typ string }

const binaryCheckpointPrefix = "b64:"

// checkpointValue renders raw key bytes into a JSON-safe durable cursor. Textual
// MySQL keys already arrive in their comparison form; binary keys need an explicit
// encoding because encoding/json replaces invalid UTF-8 bytes in Go strings.
func (p pkColumn) checkpointValue(raw []byte) string {
	if isBinaryType(p.typ) {
		return binaryCheckpointPrefix + base64.RawStdEncoding.EncodeToString(raw)
	}
	return string(raw)
}

// bindValue reverses checkpointValue for SQL comparisons. A legacy binary
// checkpoint without the prefix is bound as its original bytes when possible.
func (p pkColumn) bindValue(value string) any {
	if isIntType(p.typ) && isUnsignedType(p.typ) {
		if n, err := strconv.ParseUint(value, 10, 64); err == nil {
			// Bind an actual uint64. MySQL's binary prepared-statement protocol
			// otherwise gives a string parameter signed comparison semantics at
			// values above MaxInt64 even inside CAST(... AS UNSIGNED).
			return n
		}
	}
	if !isBinaryType(p.typ) {
		return value
	}
	if encoded, ok := strings.CutPrefix(value, binaryCheckpointPrefix); ok {
		if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
			return decoded
		}
	}
	return []byte(value)
}

// quoteIdent renders s as a backtick-quoted MySQL identifier.
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// splitHostPort splits a go-sql-driver Addr ("host:port", port optional) for the
// replication client.
func splitHostPort(addr string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("split %q: %w", addr, err)
	}
	n, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || n == 0 {
		return "", 0, fmt.Errorf("invalid port in %q", addr)
	}
	return host, uint16(n), nil
}

// Schema returns the column schema of one table (COLUMN_TYPE kept verbatim in
// Native for a same-engine round trip, classified into a LogicalType) plus the
// primary key. It implements filament.SchemaProvider so a Schematized sink can
// build matching typed tables.
func (s *Source) Schema(ctx context.Context, resource string) (rowmodel.Schema, error) {
	cols, pks, err := s.tableMeta(ctx, resource)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	return newRowDecoder(resource, cols, pks).schema, nil
}

// engine names this source in the schemas it reports.
const engine = "mysql"

// Teardown closes the pool.
func (s *Source) Teardown(context.Context) error {
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	s.binlogTLS = nil
	return nil
}
