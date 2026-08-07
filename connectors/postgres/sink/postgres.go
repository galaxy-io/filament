package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
)

// Sink loads each resource into its own typed table with native columns. The engine
// calls EnsureSchema(resource, schema) up front — Sink builds one typed table per
// resource — then Write casts each batch's jsonb payloads into those columns
// server-side via jsonb_to_recordset. It implements filament.Schematized; the engine only
// runs schema discovery for sinks that do.
type Sink struct {
	pool      *pgxpool.Pool
	run       filament.RunID
	schema    string
	dsn       string
	resumable bool
	written   atomic.Int64

	// tables is populated entirely during the engine's pre-extract EnsureSchema pass
	// (sequential), then only read by concurrent Write calls — no lock needed.
	tables map[string]*table
}

// table is one resource's ensured destination: the sanitized table identifier and the
// prebuilt INSERT … SELECT … FROM jsonb_to_recordset statement.
type table struct {
	qualified string
	insertSQL string
	deleteSQL string
}

const defaultSchema = "public"

// New returns an unconfigured sink. Open wires it to the database.
func New() *Sink { return &Sink{schema: defaultSchema} }

var (
	_ filament.Sink        = (*Sink)(nil)
	_ filament.Schematized = (*Sink)(nil)
)

// Spec describes the sink's config fields and write capabilities.
func (t *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "postgres",
		DisplayName:  "PostgreSQL",
		Description:  "Popular open-source relational database management system known for reliability and advanced features.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-postgres-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-postgres-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "dsn", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "PostgreSQL connection string"},
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name."},
			{Name: "mode", Type: filament.FieldEnum, Default: "typed", Enum: []filament.EnumOption{{Value: "typed", Label: "Typed"}}, Scope: filament.ScopePipeline, Help: "Destination table mode"},
		}},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized: true,
			Upsertable:  true,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionSnapshotReplace,
				filament.IngestionSnapshotAppend,
				filament.IngestionSnapshotUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDC,
			),
		},
	}
}

// Name identifies this sink implementation.
func (t *Sink) Name() string { return "postgres" }

// Open reads dsn/schema and opens a pool sized for the run's write parallelism. It
// does no DDL — tables are created per resource by EnsureSchema before extraction.
func (t *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	t.dsn = cfg.Secret("dsn")
	if t.dsn == "" {
		return fmt.Errorf("postgres sink: dsn is required")
	}
	if v := cfg.String("schema"); v != "" {
		t.schema = v
	}
	t.run = run.Run
	t.resumable = run.IngestionType == filament.IngestionSnapshotUpsert ||
		run.IngestionType == filament.IngestionIncrementalUpsert ||
		run.IngestionType == filament.IngestionCDC
	t.written.Store(0)
	t.tables = map[string]*table{}

	poolCfg, err := pgxpool.ParseConfig(t.dsn)
	if err != nil {
		return fmt.Errorf("postgres sink: parse dsn: %w", err)
	}
	if n := run.Options.SnapshotParallelism; n > 0 && n <= math.MaxInt32 {
		poolCfg.MaxConns = max(poolCfg.MaxConns, int32(n))
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("postgres sink: open pool: %w", err)
	}
	t.pool = pool
	if _, err := t.pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+pgx.Identifier{t.schema}.Sanitize()); err != nil {
		t.release()
		return fmt.Errorf("postgres sink: create schema %q: %w", t.schema, err)
	}
	return nil
}

// Apply validates the batch against the run's write policy, then delegates to Write.
func (t *Sink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace, filament.WriteAppend:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.Write(ctx, b)
	case filament.WriteUpsert:
		if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.Write(ctx, b)
	case filament.WriteMerge:
		if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.writeMerge(ctx, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// EnsureSchema creates resource's typed table from schema (Postgres column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and caches the
// per-resource INSERT statement. A pre-existing table gains any new columns
// (ADD COLUMN IF NOT EXISTS); an incompatible existing column type surfaces later as
// a cast error on Write (full type-change handling is deferred to schema evolution).
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema filament.RecordSchema) error {
	if t.pool == nil {
		return fmt.Errorf("postgres sink: ensure schema before open")
	}
	qualified := pgx.Identifier{t.schema, resource}.Sanitize()

	cols := make([]string, len(schema.Fields))      // "name" native [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields))    // sanitized names for the column list
	recordset := make([]string, len(schema.Fields)) // "name" native for jsonb_to_recordset
	for i, f := range schema.Fields {
		id := pgx.Identifier{f.Name}.Sanitize()
		typ := postgresColumnType(f)
		def := id + " " + typ
		idents[i] = id
		recordset[i] = id + " " + typ
		if !f.Nullable {
			def += " NOT NULL"
		}
		cols[i] = def
	}

	ddl := "CREATE TABLE IF NOT EXISTS " + qualified + " (\n\t" + strings.Join(cols, ",\n\t")
	if len(schema.PrimaryKey) > 0 {
		pk := make([]string, len(schema.PrimaryKey))
		for i, c := range schema.PrimaryKey {
			pk[i] = pgx.Identifier{c}.Sanitize()
		}
		ddl += ",\n\tPRIMARY KEY (" + strings.Join(pk, ", ") + ")"
	}
	ddl += "\n)"
	if _, err := t.pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create %s: %w", qualified, err)
	}
	// A resumable run upserts by key and must preserve any partial load from a prior
	// attempt, so it skips the full-snapshot TRUNCATE (kept only when we cannot dedup:
	// a non-resumable run, or a keyless table that re-reads whole on resume).
	upsert := t.resumable && len(schema.PrimaryKey) > 0
	if !upsert {
		// TRUNCATE before any ADD COLUMN so a NOT NULL add lands on an empty table.
		if _, err := t.pool.Exec(ctx, "TRUNCATE "+qualified); err != nil {
			return fmt.Errorf("truncate %s: %w", qualified, err)
		}
	}
	for i := range schema.Fields {
		if _, err := t.pool.Exec(ctx, "ALTER TABLE "+qualified+" ADD COLUMN IF NOT EXISTS "+cols[i]); err != nil {
			return fmt.Errorf("add column on %s: %w", qualified, err)
		}
	}

	t.tables[resource] = &table{
		qualified: qualified,
		insertSQL: fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM jsonb_to_recordset($1::jsonb) AS x(%s)%s",
			qualified, strings.Join(idents, ", "), strings.Join(idents, ", "), strings.Join(recordset, ", "),
			onConflict(upsert, schema)),
		deleteSQL: deleteUsingSQL(qualified, schema),
	}
	return nil
}

// deleteUsingSQL expands a delete run's before images on primary-key columns
// only, then deletes matching destination rows in one statement.
func deleteUsingSQL(qualified string, schema filament.RecordSchema) string {
	if len(schema.PrimaryKey) == 0 {
		return ""
	}
	fields := make(map[string]filament.SchemaField, len(schema.Fields))
	for _, field := range schema.Fields {
		fields[field.Name] = field
	}
	recordset := make([]string, len(schema.PrimaryKey))
	where := make([]string, len(schema.PrimaryKey))
	for i, name := range schema.PrimaryKey {
		id := pgx.Identifier{name}.Sanitize()
		recordset[i] = id + " " + postgresColumnType(fields[name])
		where[i] = "t." + id + " = x." + id
	}
	return fmt.Sprintf("DELETE FROM %s AS t USING jsonb_to_recordset($1::jsonb) AS x(%s) WHERE %s",
		qualified, strings.Join(recordset, ", "), strings.Join(where, " AND "))
}

func postgresColumnType(f filament.SchemaField) string {
	switch f.Logical {
	case filament.LogicalBool:
		return "boolean"
	case filament.LogicalInt16:
		return "smallint"
	case filament.LogicalInt32:
		return "integer"
	case filament.LogicalInt64:
		return "bigint"
	case filament.LogicalFloat32:
		return "real"
	case filament.LogicalFloat64:
		return "double precision"
	case filament.LogicalDecimal:
		if f.Native != "" {
			return f.Native
		}
		return "numeric"
	case filament.LogicalString:
		return "text"
	case filament.LogicalBytes:
		return "bytea"
	case filament.LogicalDate:
		return "date"
	case filament.LogicalTime:
		return "time"
	case filament.LogicalTimestamp:
		return "timestamp"
	case filament.LogicalTimestampTZ:
		return "timestamptz"
	case filament.LogicalJSON:
		return "jsonb"
	case filament.LogicalUUID:
		return "uuid"
	case filament.LogicalArray:
		if f.Native != "" {
			return f.Native
		}
		return "text[]"
	default:
		if f.Native != "" {
			return f.Native
		}
		return "text"
	}
}

// onConflict builds the ON CONFLICT clause that makes a resumable Write idempotent:
// re-delivered rows (an at-least-once resume re-reads past the last persisted cursor)
// upsert by primary key instead of erroring on the unique constraint. Non-resumable
// loads keep plain INSERT semantics (empty clause).
func onConflict(upsert bool, schema filament.RecordSchema) string {
	if !upsert {
		return ""
	}
	pkSet := make(map[string]bool, len(schema.PrimaryKey))
	pk := make([]string, len(schema.PrimaryKey))
	for i, c := range schema.PrimaryKey {
		pk[i] = pgx.Identifier{c}.Sanitize()
		pkSet[c] = true
	}
	var sets []string
	for _, f := range schema.Fields {
		if pkSet[f.Name] {
			continue
		}
		id := pgx.Identifier{f.Name}.Sanitize()
		sets = append(sets, id+" = excluded."+id)
	}
	if len(sets) == 0 {
		return " ON CONFLICT (" + strings.Join(pk, ", ") + ") DO NOTHING"
	}
	return " ON CONFLICT (" + strings.Join(pk, ", ") + ") DO UPDATE SET " + strings.Join(sets, ", ")
}

// Write casts one batch's jsonb payloads into the resource's typed columns. The
// batch's record blobs (each already valid JSON from the source) are framed as a
// single jsonb array and expanded server-side by jsonb_to_recordset, coercing every
// value to its column type. WriteCRC is over the records, unchanged — the integrity
// check is identical to the landing sink's.
func (t *Sink) Write(ctx context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	if t.pool == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write before open")
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: no schema ensured for resource %q", b.Resource)
	}

	// Frame the batch as one JSON array: [<data>,<data>,…]. Each Data is already
	// valid JSON (to_jsonb), so this is pure concatenation.
	var nbytes int64
	buf := make([]byte, 0, batchBufHint(b.Records))
	buf = append(buf, '[')
	for i := range b.Records {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, b.Records[i].Data...)
		nbytes += int64(len(b.Records[i].Data))
	}
	buf = append(buf, ']')

	tag, err := t.pool.Exec(ctx, tbl.insertSQL, string(buf))
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	n := tag.RowsAffected()
	t.written.Add(n)

	crc, _ := filament.CRC32C(b.Records)
	return filament.WriteReceipt{
		URI:      fmt.Sprintf("postgres://%s.%s", t.schema, b.Resource),
		Bytes:    nbytes,
		Rows:     int(n),
		WriteCRC: crc,
	}, nil
}

// writeMerge applies CDC records in arrival order. Maximal delete/non-delete
// runs become batched statements, and one database transaction makes the whole
// Filament batch atomic before its WAL checkpoint can be committed.
func (t *Sink) writeMerge(ctx context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	if t.pool == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write before open")
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: no schema ensured for resource %q", b.Resource)
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: begin merge: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var nbytes int64
	rows := 0
	recs := b.Records
	for len(recs) > 0 {
		deleting := recs[0].Op == filament.OpDelete
		n := 1
		for n < len(recs) && (recs[n].Op == filament.OpDelete) == deleting {
			n++
		}
		run := recs[:n]
		recs = recs[n:]
		buf, runBytes := frameJSONArray(run)
		nbytes += runBytes
		stmt := tbl.insertSQL
		if deleting {
			stmt = tbl.deleteSQL
			if stmt == "" {
				return filament.WriteReceipt{}, fmt.Errorf("postgres sink: merge delete on keyless resource %q", b.Resource)
			}
		}
		if _, err := tx.Exec(ctx, stmt, string(buf)); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: merge %s seq %d: %w", b.Resource, b.Seq, err)
		}
		rows += len(run)
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: commit merge %s seq %d: %w", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	crc, _ := filament.CRC32C(b.Records)
	return filament.WriteReceipt{
		URI: fmt.Sprintf("postgres://%s.%s", t.schema, b.Resource), Bytes: nbytes,
		Rows: rows, WriteCRC: crc,
	}, nil
}

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

// batchBufHint sizes the JSON-array scratch: the payload bytes plus separators and
// brackets, so the common case appends without growing.
func batchBufHint(recs []filament.Record) int {
	n := 2 + len(recs) // brackets + commas
	for i := range recs {
		n += len(recs[i].Data)
	}
	return n
}

// Commit releases the pool; the INSERTs are already durable.
func (t *Sink) Commit(context.Context) error {
	t.release()
	return nil
}

// Abort empties every ensured table (the run failed, so leave a clean-empty state
// rather than a half-load) and releases the pool. Best-effort on a cancel-free
// context so cleanup runs even when the failure cancelled ctx.
func (t *Sink) Abort(ctx context.Context) error {
	defer t.release()
	if t.pool == nil {
		return nil
	}
	// A resumable run keeps its partial load so a later resume can finish it — don't
	// truncate. (The engine also skips Abort on a resumable failure; this guards any
	// other Abort path.)
	if t.resumable {
		return nil
	}
	cleanup := context.WithoutCancel(ctx)
	for _, tbl := range t.tables {
		_, _ = t.pool.Exec(cleanup, "TRUNCATE "+tbl.qualified)
	}
	return nil
}

func (t *Sink) release() {
	if t.pool != nil {
		t.pool.Close()
		t.pool = nil
	}
}
