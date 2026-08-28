package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"strings"
	"sync/atomic"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	mysqlconnection "github.com/galaxy-io/filament/connectors/mysql/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
)

// Sink loads each resource into its own typed table with native columns. The engine
// calls EnsureSchema(resource, schema) up front, Sink builds one typed table per
// resource, then each batch is streamed straight into it with LOAD DATA LOCAL
// INFILE (REPLACE for upsert; a per-connection temp table of keys for merge
// deletes). The server must allow local_infile. It implements
// filament.Schematized; the engine only runs schema discovery for sinks that do.
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

// table is one resource's ensured destination: its identifiers, the LOAD DATA
// renderers, and the merge path's key temp table and delete. resumable marks a
// table whose writes are idempotent by key, so a resume must preserve its rows.
type table struct {
	qualified string
	idents    []string // quoted column names, schema order
	keyIdx    []int    // primary-key positions in schema order
	resumable bool
	rows      *loader // LOAD DATA renderer over every column
	json      []bool  // columns of type json, loaded through a utf8mb4 conversion

	// The merge path's key temp table: its renderer, identifiers, DDL and the
	// join delete. Empty for a keyless table.
	keys                             *loader
	keyIdents                        []string
	keysTemp, keysTempSQL, deleteSQL string
}

// resumableFor reports whether one resource's writes are idempotent by key —
// an upsert or CDC merge under its bound write policy.
func (t *Sink) resumableFor(resource string) bool {
	mode := t.modeFor(resource)
	return mode == filament.WriteUpsert || mode == filament.WriteMerge
}

func (t *Sink) modeFor(resource string) filament.WriteMode {
	p, ok := t.policies[resource]
	if !ok {
		p = t.policies[""]
	}
	return p.Capability.Mode
}

// New returns an unconfigured sink. Open wires it to the database.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Spec describes the sink's config fields and write capabilities.
func (t *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "mysql",
		DisplayName:  "MySQL",
		Description:  "Widely-used open-source relational database known for speed, reliability, and ease of use.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-mysql-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-mysql-light.svg",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: append(mysqlconnection.Fields(), []filament.ConfigField{
			{Name: "database", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Destination database. Empty defaults to the normalized source connection name."},
			{Name: "mode", Type: filament.FieldEnum, Default: "typed", Enum: []filament.EnumOption{{Value: "typed", Label: "Typed"}}, Scope: filament.ScopePipeline, Help: "Destination table mode"},
		}...)},
		SchemaField: "database",
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			EncodedIntegrity:   true,
			PreferredBatchRows: 4096,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDC,
				filament.IngestionCDCAppend,
			),
		},
	}
}

// Name identifies this sink implementation.
func (t *Sink) Name() string { return "mysql" }

// Validate checks connection syntax without opening a network connection.
func (t *Sink) Validate(cfg filament.Config) error {
	if _, err := mysqlconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("mysql sink: connection config: %w", err)
	}
	return nil
}

// TestConnection pings MySQL through a short-lived pool. It intentionally
// clears the database name so validating a new destination does not require
// that Open's CREATE DATABASE step has already run.
func (t *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	mc, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql sink: connection config: %w", err)
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
	mc, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql sink: connection config: %w", err)
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
	// The source renders timestamps in UTC; a same-engine timestamp column must
	// read them back in the same zone.
	if mc.Params == nil {
		mc.Params = map[string]string{}
	}
	mc.Params["time_zone"] = "'+00:00'"
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

// Apply validates the batch against the run's write policy, then routes it:
// replace and append load into the table, upsert loads with REPLACE, merge
// applies the change stream in order.
func (t *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if t.db == nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write before open")
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: no schema ensured for resource %q", b.Resource)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.write(ctx, tbl, b, false)
	case filament.WriteAppend:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.write(ctx, tbl, b, false)
	case filament.WriteUpsert:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.write(ctx, tbl, b, true)
	case filament.WriteMerge:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.writeMerge(ctx, tbl, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// EnsureSchema creates resource's typed table from schema (MySQL column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and prepares
// the resource's load and delete statements. A pre-existing table gains any new
// columns; an incompatible existing column type surfaces later as a load error
// (full type-change handling is deferred to schema evolution).
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if t.db == nil {
		return fmt.Errorf("mysql sink: ensure schema before open")
	}
	qualified := quoteIdent(t.database) + "." + quoteIdent(resource)
	same := schema.Engine == engine

	cols := make([]string, len(schema.Fields))   // `name` type [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields)) // quoted names
	types := make([]string, len(schema.Fields))
	for i, f := range schema.Fields {
		idents[i] = quoteIdent(f.Name)
		types[i] = columnType(f, same)
		cols[i] = idents[i] + " " + types[i]
		if !f.Nullable {
			cols[i] += " NOT NULL"
		}
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
	mode := t.modeFor(resource)
	resumable := t.resumableFor(resource)
	if mode == filament.WriteReplace {
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

	// The batches' Arrow schema is a pure function of the RecordSchema, so the
	// renderers are built here once rather than per batch.
	as := arrowbatch.Schema(schema)
	all := make([]int, len(schema.Fields))
	for i := range all {
		all[i] = i
	}
	tbl := &table{qualified: qualified, idents: idents, resumable: resumable || mode == filament.WriteAppend, rows: newLoader(as, all), json: make([]bool, len(types))}
	for i, typ := range types {
		tbl.json[i] = strings.EqualFold(strings.TrimSpace(typ), "json")
	}
	for _, k := range schema.PrimaryKey {
		for i, f := range schema.Fields {
			if f.Name == k {
				tbl.keyIdx = append(tbl.keyIdx, i)
			}
		}
	}
	if len(tbl.keyIdx) > 0 {
		tbl.keys = newLoader(as, tbl.keyIdx)
		tbl.keysTemp = quoteIdent("_filament_" + resource + "_keys")
		keyDefs := make([]string, len(tbl.keyIdx))
		on := make([]string, len(tbl.keyIdx))
		for n, i := range tbl.keyIdx {
			tbl.keyIdents = append(tbl.keyIdents, idents[i])
			keyDefs[n] = idents[i] + " " + types[i]
			on[n] = "t." + idents[i] + " = k." + idents[i]
		}
		tbl.keysTempSQL = "CREATE TEMPORARY TABLE IF NOT EXISTS " + tbl.keysTemp + " (" + strings.Join(keyDefs, ", ") + ")"
		tbl.deleteSQL = "DELETE t FROM " + qualified + " t JOIN " + tbl.keysTemp + " k ON " + strings.Join(on, " AND ")
	}
	t.tables[resource] = tbl
	return nil
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

// execer runs statements on one session: a connection or a transaction, so
// SHOW WARNINGS reads the statement just run.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// load streams rows [lo, hi) into the table (or, keysOnly, its keys temp table).
// LOCAL INFILE implies IGNORE: the server turns duplicate keys and bad values
// into skipped rows and coerced values with a warning, so the load checks the
// count of rows landed and the session's warnings and fails on either. Returns
// the rows loaded and payload bytes.
func (tbl *table) load(ctx context.Context, x execer, b *arrowbatch.Batch, lo, hi int, replace, keysOnly bool, integrity *writeIntegrity) (int, int64, error) {
	rows := b.Rows()
	if int(rows.NumCols()) != len(tbl.idents) {
		return 0, 0, fmt.Errorf("mysql sink: %s seq %d has %d columns, table has %d", b.Resource, b.Seq, rows.NumCols(), len(tbl.idents))
	}
	l, target, idents, json := tbl.rows, tbl.qualified, tbl.idents, tbl.json
	if keysOnly {
		l, target, idents, json = tbl.keys, tbl.keysTemp, tbl.keyIdents, nil
	}
	payload, expectedCRC := l.encode(rows, lo, hi)
	integrity.arrow = b.IntegrityCRC()
	integrity.encoded = crc32.Update(integrity.encoded, loadCRCTable, payload)
	name, done := register(payload)
	defer done()
	if err := verifyLoadChecksum(payload, expectedCRC); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: verify load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	res, err := x.ExecContext(ctx, loadSQL(name, target, idents, json, replace))
	if err != nil {
		return 0, 0, fmt.Errorf("mysql sink: load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	if !replace { // with REPLACE a duplicate is the point and counts twice
		if n, err := res.RowsAffected(); err == nil && n != int64(hi-lo) {
			return 0, 0, fmt.Errorf("mysql sink: load %s seq %d: %d of %d rows landed (duplicate keys?)", b.Resource, b.Seq, n, hi-lo)
		}
	}
	if err := firstWarning(ctx, x); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	return hi - lo, int64(len(payload)), nil
}

// firstWarning returns the session's first warning from the statement just run
// as an error, or nil when there is none.
func firstWarning(ctx context.Context, x execer) error {
	rows, err := x.QueryContext(ctx, "SHOW WARNINGS LIMIT 1")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return rows.Err()
	}
	var level, msg string
	var code int
	if err := rows.Scan(&level, &code, &msg); err != nil {
		return err
	}
	return fmt.Errorf("%s %d: %s", strings.ToLower(level), code, msg)
}

// write loads the whole batch; replace makes duplicate keys overwrite, the
// idempotent upsert an at-least-once resume relies on.
func (t *Sink) write(ctx context.Context, tbl *table, b *arrowbatch.Batch, replace bool) (filament.WriteReceipt, error) {
	conn, err := t.db.Conn(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write %s: %w", b.Resource, err)
	}
	defer func() { _ = conn.Close() }()
	var integrity writeIntegrity
	rows, nbytes, err := tbl.load(ctx, conn, b, 0, b.NumRows(), replace, false, &integrity)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes, integrity), nil
}

// writeMerge applies a CDC batch: rows split into maximal same-kind runs in
// arrival order — a delete of a key must not jump over its re-insert — and each
// run lands as one statement: inserts/updates through LOAD DATA REPLACE, deletes
// through the keys temp table and a join delete. One transaction on one
// connection (the temp table is per session) makes the batch atomic.
func (t *Sink) writeMerge(ctx context.Context, tbl *table, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if tbl.deleteSQL == "" {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: merge on keyless resource %q", b.Resource)
	}
	conn, err := t.db.Conn(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: merge %s: %w", b.Resource, err)
	}
	defer func() { _ = conn.Close() }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: begin merge %s: %w", b.Resource, err)
	}
	defer func() { _ = tx.Rollback() }()

	var nbytes int64
	rows := 0
	var integrity writeIntegrity
	n := b.NumRows()
	for lo := 0; lo < n; {
		deleting := b.Op(lo) == rowmodel.OpDelete
		hi := lo + 1
		for hi < n && (b.Op(hi) == rowmodel.OpDelete) == deleting {
			hi++
		}
		var (
			k   int
			nb  int64
			err error
		)
		if deleting {
			k, nb, err = t.deleteRange(ctx, tx, tbl, b, lo, hi, &integrity)
		} else {
			k, nb, err = tbl.load(ctx, tx, b, lo, hi, true, false, &integrity)
		}
		if err != nil {
			return filament.WriteReceipt{}, err
		}
		rows += k
		nbytes += nb
		lo = hi
	}
	if err := tx.Commit(); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: commit merge %s seq %d: %w", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes, integrity), nil
}

// deleteRange loads the keys of rows [lo, hi) into the session's keys temp table
// and deletes the matching destination rows.
func (t *Sink) deleteRange(ctx context.Context, tx *sql.Tx, tbl *table, b *arrowbatch.Batch, lo, hi int, integrity *writeIntegrity) (int, int64, error) {
	if _, err := tx.ExecContext(ctx, tbl.keysTempSQL); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: keys temp table %s: %w", b.Resource, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+tbl.keysTemp); err != nil { //nolint:gosec // identifier backtick-quoted via quoteIdent
		return 0, 0, fmt.Errorf("mysql sink: clear keys %s: %w", b.Resource, err)
	}
	k, nb, err := tbl.load(ctx, tx, b, lo, hi, false, true, integrity)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, tbl.deleteSQL); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: delete %s seq %d: %w", b.Resource, b.Seq, err)
	}
	return k, nb, nil
}

type writeIntegrity struct {
	arrow   uint32
	encoded uint32
}

func (t *Sink) receipt(b *arrowbatch.Batch, rows int, nbytes int64, integrity writeIntegrity) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("mysql://%s.%s", t.database, b.Resource),
		Bytes:      nbytes,
		Rows:       rows,
		WriteCRC:   integrity.arrow,
		EncodedCRC: &integrity.encoded,
	}
}

// Commit releases the pool; the loads are already durable (autocommit).
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
