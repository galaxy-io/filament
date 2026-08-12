package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/go-sql-driver/mysql"

	"github.com/galaxy-io/filament"
)

// Sink loads each resource into its own typed table with native columns. The engine
// calls EnsureSchema(resource, schema) up front, Sink builds one typed table per
// resource then Write casts each batch's JSON payloads into those columns
// server-side via JSON_TABLE (MySQL 8.0+), the analog of the Postgres sink's
// jsonb_to_recordset. It implements filament.Schematized; the engine only runs
// schema discovery for sinks that do.
type Sink struct {
	db       *sql.DB
	run      filament.RunID
	database string
	policies map[string]filament.WritePolicy
	written  atomic.Int64

	// tables is populated entirely during the engine's pre-extract EnsureSchema pass
	// (sequential), then only read by concurrent Write calls — no lock needed.
	tables map[string]*table
}

// table is one resource's ensured destination: the quoted table identifier and the
// prebuilt INSERT … SELECT … FROM JSON_TABLE statement, plus the JSON_TABLE-join
// DELETE used by merge (CDC) writes on a keyed table. resumable marks a table
// whose writes are idempotent by key, so a resume must preserve its rows.
type table struct {
	qualified string
	insertSQL string
	deleteSQL string // empty for a keyless table
	resumable bool
}

// resumableFor reports whether one resource's writes are idempotent by key —
// an upsert or CDC merge under its bound write policy.
func (t *Sink) resumableFor(resource string) bool {
	p, ok := t.policies[resource]
	if !ok {
		p = t.policies[""]
	}
	return p.Capability.Mode == filament.WriteUpsert || p.Capability.Mode == filament.WriteMerge
}

// New returns an unconfigured sink. Open wires it to the database.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink            = (*Sink)(nil)
	_ filament.LiveValidatable = (*Sink)(nil)
	_ filament.Schematized     = (*Sink)(nil)
)

// Spec describes the sink's config fields and write capabilities.
func (t *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "mysql",
		DisplayName:  "MySQL",
		Description:  "Widely-used open-source relational database known for speed, reliability, and ease of use.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-mysql-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-mysql-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "dsn", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "MySQL connection string (user:pass@tcp(host:port)/dbname)"},
			{Name: "database", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Destination database. Empty defaults to the normalized source connection name."},
			{Name: "mode", Type: filament.FieldEnum, Default: "typed", Enum: []filament.EnumOption{{Value: "typed", Label: "Typed"}}, Scope: filament.ScopePipeline, Help: "Destination table mode"},
		}},
		SchemaField: "database",
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			PreferredBatchRows: 4096,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDC,
			),
		},
	}
}

// Name identifies this sink implementation.
func (t *Sink) Name() string { return "mysql" }

// TestConnection pings MySQL through a short-lived pool. It intentionally
// clears the database name so validating a new destination does not require
// that Open's CREATE DATABASE step has already run.
func (t *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	dsn := cfg.Secret("dsn")
	if dsn == "" {
		return fmt.Errorf("mysql sink: dsn is required")
	}
	mc, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("mysql sink: parse dsn: %w", err)
	}
	mc.DBName = ""
	db, err := sql.Open("mysql", mc.FormatDSN())
	if err != nil {
		return fmt.Errorf("mysql sink: open: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql sink: ping: %w", err)
	}
	return nil
}

// Open reads dsn/database and opens a pool sized for the run's write parallelism. It
// does no DDL beyond CREATE DATABASE — tables are created per resource by
// EnsureSchema before extraction.
func (t *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	dsn := cfg.Secret("dsn")
	if dsn == "" {
		return fmt.Errorf("mysql sink: dsn is required")
	}
	mc, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("mysql sink: parse dsn: %w", err)
	}
	t.database = mc.DBName
	if v := cfg.String("database"); v != "" {
		t.database = v
	}
	if t.database == "" {
		return fmt.Errorf("mysql sink: dsn has no database and \"database\" is unset")
	}
	t.run = run.Run
	t.policies = run.WritePolicies
	t.written.Store(0)
	t.tables = map[string]*table{}

	bootstrap := *mc
	bootstrap.DBName = ""
	bdb, err := sql.Open("mysql", bootstrap.FormatDSN())
	if err != nil {
		return fmt.Errorf("mysql sink: open: %w", err)
	}
	_, err = bdb.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteIdent(t.database))
	_ = bdb.Close()
	if err != nil {
		return fmt.Errorf("mysql sink: create database %q: %w", t.database, err)
	}

	mc.DBName = t.database
	db, err := sql.Open("mysql", mc.FormatDSN())
	if err != nil {
		return fmt.Errorf("mysql sink: open: %w", err)
	}
	if n := run.Options.SnapshotParallelism; n > 0 {
		db.SetMaxOpenConns(max(n, 4))
	}
	t.db = db
	return nil
}

// Apply validates the batch against the run's write policy, then delegates to
// Write (or writeMerge for CDC).
func (t *Sink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace, filament.WriteAppend:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.Write(ctx, b)
	case filament.WriteUpsert:
		if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.Write(ctx, b)
	case filament.WriteMerge:
		if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.writeMerge(ctx, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// EnsureSchema creates resource's typed table from schema (MySQL column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and caches the
// per-resource INSERT statement. A pre-existing table gains any new columns; an
// incompatible existing column type surfaces later as a cast error on Write (full
// type-change handling is deferred to schema evolution).
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema filament.RecordSchema) error {
	if t.db == nil {
		return fmt.Errorf("mysql sink: ensure schema before open")
	}
	qualified := quoteIdent(t.database) + "." + quoteIdent(resource)

	cols := make([]string, len(schema.Fields))    // `name` native [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields))  // quoted names for the column list
	jtCols := make([]string, len(schema.Fields))  // JSON_TABLE COLUMNS clauses
	selects := make([]string, len(schema.Fields)) // projection over the JSON_TABLE rows
	for i, f := range schema.Fields {
		id := quoteIdent(f.Name)
		typ := mysqlColumnType(f)
		def := id + " " + typ
		if !f.Nullable {
			def += " NOT NULL"
		}
		cols[i] = def
		idents[i] = id
		jtCols[i], selects[i] = jsonTableColumn(f, id)
	}

	ddl := "CREATE TABLE IF NOT EXISTS " + qualified + " (\n\t" + strings.Join(cols, ",\n\t") //nolint:gosec // identifiers backtick-quoted via quoteIdent; no user values
	if len(schema.PrimaryKey) > 0 {
		pk := make([]string, len(schema.PrimaryKey))
		for i, c := range schema.PrimaryKey {
			pk[i] = quoteIdent(c)
		}
		ddl += ",\n\tPRIMARY KEY (" + strings.Join(pk, ", ") + ")"
	}
	ddl += "\n)"
	if _, err := t.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create %s: %w", qualified, err)
	}
	// A resumable table upserts by key and must preserve any partial load from a prior
	// attempt, so it skips the full-snapshot TRUNCATE (kept only when we cannot dedup:
	// a non-resumable table, or a keyless table that re-reads whole on resume).
	resumable := t.resumableFor(resource)
	upsert := resumable && len(schema.PrimaryKey) > 0
	if !upsert {
		// TRUNCATE before any ADD COLUMN so a NOT NULL add lands on an empty table.
		if _, err := t.db.ExecContext(ctx, "TRUNCATE "+qualified); err != nil {
			return fmt.Errorf("truncate %s: %w", qualified, err)
		}
	}
	// MySQL has no ADD COLUMN IF NOT EXISTS; add only the columns the live table lacks.
	existing, err := t.columnSet(ctx, resource)
	if err != nil {
		return fmt.Errorf("columns %s: %w", qualified, err)
	}
	for i, f := range schema.Fields {
		if existing[f.Name] {
			continue
		}
		if _, err := t.db.ExecContext(ctx, "ALTER TABLE "+qualified+" ADD COLUMN "+cols[i]); err != nil {
			return fmt.Errorf("add column on %s: %w", qualified, err)
		}
	}

	// The batch rows come out of JSON_TABLE, get projected (FROM_BASE64 etc.) in a
	// derived table aliased `new`, and insert from there. ON DUPLICATE KEY UPDATE
	// references the derived table for the incoming values; a VALUES row alias
	// cannot serve that role here because it is only valid on INSERT … VALUES,
	// not INSERT … SELECT.
	t.tables[resource] = &table{
		qualified: qualified,
		insertSQL: fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM (SELECT %s FROM JSON_TABLE(?, '$[*]' COLUMNS (%s)) AS x) AS new%s",
			qualified, strings.Join(idents, ", "), strings.Join(idents, ", "), strings.Join(selects, ", "), strings.Join(jtCols, ", "),
			onDuplicate(upsert, schema)),
		deleteSQL: deleteJoinSQL(qualified, schema),
		resumable: resumable,
	}
	return nil
}

// deleteJoinSQL builds the merge path's batched delete: the delete batch (a JSON
// array of before-image rows) expands through JSON_TABLE on the primary-key columns
// only, and a multi-table DELETE joined on the key removes every matched row in one
// statement. Empty for a keyless table (merge requires a key; policy validation
// enforces it upstream).
func deleteJoinSQL(qualified string, schema filament.RecordSchema) string {
	if len(schema.PrimaryKey) == 0 {
		return ""
	}
	fieldByName := make(map[string]filament.SchemaField, len(schema.Fields))
	for _, f := range schema.Fields {
		fieldByName[f.Name] = f
	}
	jtCols := make([]string, len(schema.PrimaryKey))
	on := make([]string, len(schema.PrimaryKey))
	for i, name := range schema.PrimaryKey {
		f := fieldByName[name]
		id := quoteIdent(name)
		jt, _ := jsonTableColumn(f, id)
		jtCols[i] = jt
		// A bytes key arrives base64-encoded (the source's TO_BASE64 convention);
		// decode it on the join so it compares against the binary column.
		if f.Logical == filament.LogicalBytes {
			on[i] = "t." + id + " = FROM_BASE64(j." + id + ")"
		} else {
			on[i] = "t." + id + " = j." + id
		}
	}
	return fmt.Sprintf("DELETE t FROM %s t JOIN JSON_TABLE(?, '$[*]' COLUMNS (%s)) AS j ON %s",
		qualified, strings.Join(jtCols, ", "), strings.Join(on, " AND "))
}

// columnSet returns the live column names of the destination table.
func (t *Sink) columnSet(ctx context.Context, resource string) (map[string]bool, error) {
	const q = `SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`
	rows, err := t.db.QueryContext(ctx, q, t.database, resource)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

// jsonTableColumn renders one field's JSON_TABLE COLUMNS clause and its outer SELECT
// projection. Most types extract directly at their column type; JSON stays JSON (so
// nested structure survives), and bytes come back through FROM_BASE64 (the source
// encodes binary columns with TO_BASE64 — MySQL JSON has no binary representation).
func jsonTableColumn(f filament.SchemaField, id string) (jt, sel string) {
	path := "PATH " + quotePathLiteral(f.Name)
	switch f.Logical {
	case filament.LogicalJSON:
		return id + " JSON " + path, id
	case filament.LogicalBytes:
		return id + " LONGTEXT " + path, "FROM_BASE64(" + id + ") AS " + id
	default:
		return id + " " + mysqlColumnType(f) + " " + path, id
	}
}

// quotePathLiteral renders the JSON path '$."name"' for a column, escaping for both
// the SQL string literal and the JSON path member.
func quotePathLiteral(name string) string {
	name = strings.ReplaceAll(name, `\`, `\\`)
	name = strings.ReplaceAll(name, `"`, `\"`)
	name = strings.ReplaceAll(name, "'", "''")
	return `'$."` + name + `"'`
}

// mysqlColumnType maps a portable LogicalType onto a MySQL column type, preferring
// the source's Native declaration for a same-engine round-trip.
func mysqlColumnType(f filament.SchemaField) string {
	switch f.Logical {
	case filament.LogicalBool:
		return "tinyint(1)"
	case filament.LogicalInt16:
		return "smallint"
	case filament.LogicalInt32:
		return "int"
	case filament.LogicalInt64:
		return "bigint"
	case filament.LogicalFloat32:
		return "float"
	case filament.LogicalFloat64:
		return "double"
	case filament.LogicalDecimal:
		if native := nativeIfMySQL(f.Native); native != "" {
			return native
		}
		return "decimal(38,9)"
	case filament.LogicalString:
		if native := nativeIfMySQL(f.Native); native != "" {
			return native
		}
		return "longtext"
	case filament.LogicalBytes:
		if native := nativeIfMySQL(f.Native); native != "" {
			return native
		}
		return "longblob"
	case filament.LogicalDate:
		return "date"
	case filament.LogicalTime:
		return "time(6)"
	case filament.LogicalTimestamp:
		return "datetime(6)"
	case filament.LogicalTimestampTZ:
		// timestamp's range stops at 2038; datetime holds any instant (rendered UTC).
		return "datetime(6)"
	case filament.LogicalJSON, filament.LogicalArray:
		return "json"
	case filament.LogicalUUID:
		return "char(36)"
	default:
		if native := nativeIfMySQL(f.Native); native != "" {
			return native
		}
		return "longtext"
	}
}

// nativeIfMySQL returns the field's Native type when it parses as a MySQL type name
// (a cross-engine Native like "character varying(20)" would fail DDL); empty otherwise.
func nativeIfMySQL(native string) string {
	t := strings.ToLower(strings.TrimSpace(native))
	if t == "" {
		return ""
	}
	base := t
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint",
		"decimal", "numeric", "float", "double", "real", "bit",
		"char", "varchar", "tinytext", "text", "mediumtext", "longtext",
		"binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
		"date", "datetime", "timestamp", "time", "year",
		"json", "enum", "set":
		return t
	default:
		return ""
	}
}

// onDuplicate builds the ON DUPLICATE KEY UPDATE clause that makes a resumable Write
// idempotent: re-delivered rows (an at-least-once resume re-reads past the last
// persisted cursor) upsert by primary key instead of erroring on the unique
// constraint. Non-resumable loads keep plain INSERT semantics (empty clause).
// A PK-only table degrades to a no-op update of its first key column (MySQL's
// DO NOTHING idiom).
func onDuplicate(upsert bool, schema filament.RecordSchema) string {
	if !upsert {
		return ""
	}
	pkSet := make(map[string]bool, len(schema.PrimaryKey))
	for _, c := range schema.PrimaryKey {
		pkSet[c] = true
	}
	var sets []string
	for _, f := range schema.Fields {
		if pkSet[f.Name] {
			continue
		}
		id := quoteIdent(f.Name)
		sets = append(sets, id+" = new."+id)
	}
	if len(sets) == 0 {
		id := quoteIdent(schema.PrimaryKey[0])
		sets = []string{id + " = new." + id}
	}
	return " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", ")
}

// quoteIdent renders s as a backtick-quoted MySQL identifier.
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// Write casts one batch's JSON payloads into the resource's typed columns. The
// batch's record blobs (each already valid JSON from the source) are framed as a
// single JSON array and expanded server-side by JSON_TABLE, coercing every value to
// its column type. WriteCRC is over the records, unchanged — the integrity check is
// identical to the landing sink's.
func (t *Sink) Write(ctx context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	tbl, err := t.tableForBatch(b.Resource)
	if err != nil {
		return filament.WriteReceipt{}, err
	}

	buf, nbytes := frameJSONArray(b.Records)
	res, err := t.db.ExecContext(ctx, tbl.insertSQL, string(buf))
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	// ON DUPLICATE KEY UPDATE reports 2 per updated row; the batch's logical row count
	// is its record count (every record either inserted or updated).
	n := int64(len(b.Records))
	if aff, err := res.RowsAffected(); err == nil && aff < n {
		n = aff
	}
	t.written.Add(n)

	return t.writeReceipt(b.Resource, nbytes, int(n), b.Records), nil
}

func (t *Sink) tableForBatch(resource string) (*table, error) {
	if t.db == nil {
		return nil, fmt.Errorf("mysql sink: write before open")
	}
	tbl := t.tables[resource]
	if tbl == nil {
		return nil, fmt.Errorf("mysql sink: no schema ensured for resource %q", resource)
	}
	return tbl, nil
}

func (t *Sink) writeReceipt(resource string, nbytes int64, rows int, recs []filament.Record) filament.WriteReceipt {
	crc, _ := filament.CRC32C(recs)
	return filament.WriteReceipt{
		URI:      fmt.Sprintf("mysql://%s.%s", t.database, resource),
		Bytes:    nbytes,
		Rows:     rows,
		WriteCRC: crc,
	}
}

// batchBufHint sizes the JSON-array scratch: the payload bytes plus separators and
// brackets, so the common case appends without growing.
func batchBufHint(recs []filament.Record) int {
	n := 2 + len(recs) // brackets + commas
	for i := range recs {
		n += len(recs[i].Data)
	}
	return n
}

// frameJSONArray concatenates record payloads into one JSON array: [<data>,…].
// Each Data is already valid JSON, so this is pure concatenation. Returns the
// framed buffer and the payload byte count.
func frameJSONArray(recs []filament.Record) ([]byte, int64) {
	var nbytes int64
	buf := make([]byte, 0, batchBufHint(recs))
	buf = append(buf, '[')
	for i := range recs {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, recs[i].Data...)
		nbytes += int64(len(recs[i].Data))
	}
	return append(buf, ']'), nbytes
}

// writeMerge applies a CDC batch: the records are split into maximal same-kind runs
// in arrival order — order matters, a delete of a key must not jump over its
// re-insert — and each run lands as one statement: inserts/updates through the
// upsert INSERT … JSON_TABLE, deletes through the JSON_TABLE-join DELETE keyed on
// the primary key from each delete's before-image payload. One database
// transaction makes the whole batch atomic before its checkpoint can be committed.
func (t *Sink) writeMerge(ctx context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	tbl, err := t.tableForBatch(b.Resource)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: begin merge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var nbytes int64
	rows := 0
	recs := b.Records
	for len(recs) > 0 {
		isDelete := recs[0].Op == filament.OpDelete
		n := 1
		for n < len(recs) && (recs[n].Op == filament.OpDelete) == isDelete {
			n++
		}
		run := recs[:n]
		recs = recs[n:]

		buf, runBytes := frameJSONArray(run)
		nbytes += runBytes
		stmt := tbl.insertSQL
		if isDelete {
			stmt = tbl.deleteSQL
			if stmt == "" {
				return filament.WriteReceipt{}, fmt.Errorf("mysql sink: merge delete on keyless resource %q", b.Resource)
			}
		}
		if _, err := tx.ExecContext(ctx, stmt, string(buf)); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: merge %s seq %d: %w", b.Resource, b.Seq, err)
		}
		rows += len(run)
	}
	if err := tx.Commit(); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: commit merge %s seq %d: %w", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))

	return t.writeReceipt(b.Resource, nbytes, rows, b.Records), nil
}

// Commit releases the pool; the INSERTs are already durable (autocommit).
func (t *Sink) Commit(context.Context) error {
	t.release()
	return nil
}

// Abort empties every ensured table (the run failed, so leave a clean-empty state
// rather than a half-load) and releases the pool. Best-effort on a cancel-free
// context so cleanup runs even when the failure cancelled ctx.
func (t *Sink) Abort(ctx context.Context) error {
	defer t.release()
	if t.db == nil {
		return nil
	}
	// A resumable table keeps its partial load so a later resume can finish it — don't
	// truncate. (The engine also skips Abort on a resumable failure; this guards any
	// other Abort path.)
	cleanup := context.WithoutCancel(ctx)
	for _, tbl := range t.tables {
		if tbl.resumable {
			continue
		}
		_, _ = t.db.ExecContext(cleanup, "TRUNCATE "+tbl.qualified)
	}
	return nil
}

func (t *Sink) release() {
	if t.db != nil {
		_ = t.db.Close()
		t.db = nil
	}
}
