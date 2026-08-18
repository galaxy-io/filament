package postgres

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

// Sink loads each resource into its own typed table with native columns. The engine
// calls EnsureSchema(resource, schema) up front — Sink builds one typed table per
// resource — then each batch is COPYed straight into it (replace, append) or
// through a per-connection temp table into an upsert or delete (upsert, merge). It
// implements filament.Schematized; the engine only runs schema discovery for sinks
// that do.
type Sink struct {
	pool     *pgxpool.Pool
	run      filament.RunID
	schema   string
	dsn      string
	policies map[string]filament.WritePolicy
	written  atomic.Int64

	// tables is populated entirely during the engine's pre-extract EnsureSchema pass
	// (sequential), then only read by concurrent Write calls — no lock needed.
	tables map[string]*table
}

// table is one resource's ensured destination: its identifiers, column types, the
// COPY renderers, and the prebuilt statements of the key-based paths. resumable
// marks a table whose writes are idempotent by key, so a resume must preserve its
// rows.
type table struct {
	qualified string
	idents    []string // sanitized column names, schema order
	types     []string // Postgres column types, schema order
	keyIdx    []int    // primary-key positions in schema order
	resumable bool

	mu     sync.Mutex
	copier map[*arrow.Schema]*copier // per Arrow schema seen; keys copier alongside
	keys   map[*arrow.Schema]*copier

	// Key-based paths land in a per-connection temp table (rows for the upsert,
	// keys for the delete) and fold from there; the statements are built once.
	tempSQL, upsertSQL, keysTempSQL, deleteSQL string
	temp, keysTemp                             string
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

const defaultSchema = "public"

// New returns an unconfigured sink. Open wires it to the database.
func New() *Sink { return &Sink{schema: defaultSchema} }

var (
	_ filament.Sink            = (*Sink)(nil)
	_ filament.LiveValidatable = (*Sink)(nil)
	_ filament.Schematized     = (*Sink)(nil)
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
func (t *Sink) Name() string { return "postgres" }

// TestConnection opens a short-lived pool and verifies the configured
// credentials without creating the destination schema.
func (t *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	dsn := cfg.Secret("dsn")
	if dsn == "" {
		return fmt.Errorf("postgres sink: dsn is required")
	}
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("postgres sink: parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("postgres sink: open pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres sink: ping: %w", err)
	}
	return nil
}

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
	t.policies = run.WritePolicies
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

// Apply validates the batch against the run's write policy, then routes it: replace
// and append COPY into the table, upsert folds through a temp table, merge applies
// the change stream in order.
func (t *Sink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if t.pool == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write before open")
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: no schema ensured for resource %q", b.Resource)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace, filament.WriteAppend:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateOps(b.Resource, b.Ops); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.write(ctx, tbl, b)
	case filament.WriteUpsert:
		if err := opts.Policy.ValidateOps(b.Resource, b.Ops); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.writeUpsert(ctx, tbl, b)
	case filament.WriteMerge:
		if err := opts.Policy.ValidateOps(b.Resource, b.Ops); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.writeMerge(ctx, tbl, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// EnsureSchema creates resource's typed table from schema (Postgres column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and prepares the
// resource's COPY and fold statements. A pre-existing table gains any new columns
// (ADD COLUMN IF NOT EXISTS); an incompatible existing column type surfaces later as
// a COPY error (full type-change handling is deferred to schema evolution).
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema filament.RecordSchema) error {
	if t.pool == nil {
		return fmt.Errorf("postgres sink: ensure schema before open")
	}
	qualified := pgx.Identifier{t.schema, resource}.Sanitize()
	same := schema.Engine == engine

	cols := make([]string, len(schema.Fields)) // "name" type [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields))
	types := make([]string, len(schema.Fields))
	for i, f := range schema.Fields {
		idents[i] = pgx.Identifier{f.Name}.Sanitize()
		types[i] = columnType(f, same)
		cols[i] = idents[i] + " " + types[i]
		if !f.Nullable {
			cols[i] += " NOT NULL"
		}
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
	// A resumable table upserts by key and must preserve any partial load from a prior
	// attempt, so it skips the full-snapshot TRUNCATE (kept only when we cannot dedup:
	// a non-resumable table, or a keyless table that re-reads whole on resume).
	resumable := t.resumableFor(resource)
	upsert := resumable && len(schema.PrimaryKey) > 0
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

	tbl := &table{
		qualified: qualified,
		idents:    idents,
		types:     types,
		resumable: resumable,
		copier:    map[*arrow.Schema]*copier{},
		keys:      map[*arrow.Schema]*copier{},
	}
	for _, k := range schema.PrimaryKey {
		for i, f := range schema.Fields {
			if f.Name == k {
				tbl.keyIdx = append(tbl.keyIdx, i)
			}
		}
	}
	if len(tbl.keyIdx) > 0 {
		tbl.temp = pgx.Identifier{"_filament_" + resource}.Sanitize()
		tbl.keysTemp = pgx.Identifier{"_filament_" + resource + "_keys"}.Sanitize()
		tbl.tempSQL = tempTableSQL(tbl.temp, idents, types, true)
		tbl.upsertSQL = upsertSQL(tbl)
		keyIdents, keyTypes := pick(idents, tbl.keyIdx), pick(types, tbl.keyIdx)
		tbl.keysTempSQL = tempTableSQL(tbl.keysTemp, keyIdents, keyTypes, false)
		tbl.deleteSQL = deleteSQL(tbl.qualified, tbl.keysTemp, keyIdents)
	}
	t.tables[resource] = tbl
	return nil
}

// engine is the source engine whose native type spellings this sink reuses.
const engine = "postgres"

// columnType picks a destination column type: the source's own spelling when it
// came from Postgres, else a portable mapping from the logical type.
func columnType(f filament.SchemaField, sameEngine bool) string {
	if sameEngine && f.Native != "" {
		return f.Native
	}
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
		if f.Precision > 0 {
			return fmt.Sprintf("numeric(%d,%d)", f.Precision, f.Scale)
		}
		return "numeric"
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
	default:
		return "text"
	}
}

// tempTableSQL creates the per-connection scratch table a key-based fold loads
// into: the given columns, nullable, plus _ord (row order within the batch) when
// ordered. It is emptied before every load; a merge folds several runs through
// it inside one transaction.
func tempTableSQL(name string, idents, types []string, ordered bool) string {
	cols := make([]string, len(idents), len(idents)+1)
	for i := range idents {
		cols[i] = idents[i] + " " + types[i]
	}
	if ordered {
		cols = append(cols, "_ord integer")
	}
	return "CREATE TEMP TABLE IF NOT EXISTS " + name + " (" + strings.Join(cols, ", ") + "); TRUNCATE " + name
}

// upsertSQL folds the temp table into the destination: the last row per key wins
// within the batch, then INSERT … ON CONFLICT DO UPDATE by primary key makes the
// write idempotent (an at-least-once resume re-delivers rows past the last
// persisted cursor).
func upsertSQL(tbl *table) string {
	keys := pick(tbl.idents, tbl.keyIdx)
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	var sets []string
	for _, id := range tbl.idents {
		if !keySet[id] {
			sets = append(sets, id+" = excluded."+id)
		}
	}
	cols := strings.Join(tbl.idents, ", ")
	q := fmt.Sprintf("INSERT INTO %s (%s) SELECT DISTINCT ON (%s) %s FROM %s ORDER BY %s, _ord DESC ON CONFLICT (%s) DO ",
		tbl.qualified, cols, strings.Join(keys, ", "), cols, tbl.temp, strings.Join(keys, ", "), strings.Join(keys, ", "))
	if len(sets) == 0 {
		return q + "NOTHING"
	}
	return q + "UPDATE SET " + strings.Join(sets, ", ")
}

// deleteSQL removes every destination row whose key is in the keys temp table.
func deleteSQL(qualified, keysTemp string, keyIdents []string) string {
	where := make([]string, len(keyIdents))
	for i, k := range keyIdents {
		where[i] = "t." + k + " = k." + k
	}
	return fmt.Sprintf("DELETE FROM %s AS t USING %s AS k WHERE %s", qualified, keysTemp, strings.Join(where, " AND "))
}

func pick[T any](all []T, idx []int) []T {
	out := make([]T, len(idx))
	for n, i := range idx {
		out[n] = all[i]
	}
	return out
}

// copierFor returns the batch's COPY renderer over all columns, built once per
// Arrow schema (one per source builder). Column order is the schema's; the
// destination column list is the same order.
func (tbl *table) copierFor(rows arrow.RecordBatch, keysOnly bool) *copier {
	schema := rows.Schema()
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	cache := tbl.copier
	if keysOnly {
		cache = tbl.keys
	}
	if c := cache[schema]; c != nil {
		return c
	}
	idx := make([]int, schema.NumFields())
	for i := range idx {
		idx[i] = i
	}
	types := tbl.types
	if keysOnly {
		idx, types = tbl.keyIdx, pick(tbl.types, tbl.keyIdx)
	}
	c := newCopier(schema, idx, types)
	cache[schema] = c
	return c
}

// write COPYs the whole batch into the table.
func (t *Sink) write(ctx context.Context, tbl *table, b filament.Batch) (filament.WriteReceipt, error) {
	c := tbl.copierFor(b.Rows, false)
	payload := c.encode(b.Rows, 0, b.NumRows(), false)
	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		return filament.WriteReceipt{}, copyError("acquire", b.Resource, b.Seq, err)
	}
	defer conn.Release()
	tag, err := conn.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.qualified, tbl.idents))
	if err != nil {
		return filament.WriteReceipt{}, copyError("copy", b.Resource, b.Seq, err)
	}
	t.written.Add(tag.RowsAffected())
	return t.receipt(b, int(tag.RowsAffected()), int64(len(payload))), nil
}

// writeUpsert loads the batch into the connection's temp table and folds it into
// the destination by key, in one transaction.
func (t *Sink) writeUpsert(ctx context.Context, tbl *table, b filament.Batch) (filament.WriteReceipt, error) {
	if tbl.upsertSQL == "" {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: upsert on keyless resource %q", b.Resource)
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return filament.WriteReceipt{}, copyError("begin", b.Resource, b.Seq, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	rows, nbytes, err := t.upsertRange(ctx, tx, tbl, b, 0, b.NumRows())
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.WriteReceipt{}, copyError("commit", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes), nil
}

// upsertRange COPYs rows [lo, hi) into the temp table and folds them into the
// destination. Returns rows folded and payload bytes.
func (t *Sink) upsertRange(ctx context.Context, tx pgx.Tx, tbl *table, b filament.Batch, lo, hi int) (int, int64, error) {
	if _, err := tx.Exec(ctx, tbl.tempSQL); err != nil {
		return 0, 0, copyError("temp table", b.Resource, b.Seq, err)
	}
	c := tbl.copierFor(b.Rows, false)
	payload := c.encode(b.Rows, lo, hi, true)
	idents := append(append(make([]string, 0, len(tbl.idents)+1), tbl.idents...), "_ord")
	if _, err := tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.temp, idents)); err != nil {
		return 0, 0, copyError("copy", b.Resource, b.Seq, err)
	}
	if _, err := tx.Exec(ctx, tbl.upsertSQL); err != nil {
		return 0, 0, copyError("upsert", b.Resource, b.Seq, err)
	}
	return hi - lo, int64(len(payload)), nil
}

// deleteRange COPYs the keys of rows [lo, hi) into the keys temp table and deletes
// the matching destination rows.
func (t *Sink) deleteRange(ctx context.Context, tx pgx.Tx, tbl *table, b filament.Batch, lo, hi int) (int, int64, error) {
	if _, err := tx.Exec(ctx, tbl.keysTempSQL); err != nil {
		return 0, 0, copyError("keys temp table", b.Resource, b.Seq, err)
	}
	c := tbl.copierFor(b.Rows, true)
	payload := c.encode(b.Rows, lo, hi, false)
	if _, err := tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.keysTemp, pick(tbl.idents, tbl.keyIdx))); err != nil {
		return 0, 0, copyError("copy keys", b.Resource, b.Seq, err)
	}
	if _, err := tx.Exec(ctx, tbl.deleteSQL); err != nil {
		return 0, 0, copyError("delete", b.Resource, b.Seq, err)
	}
	return hi - lo, int64(len(payload)), nil
}

// writeMerge applies CDC rows in arrival order. Maximal delete/non-delete runs
// become one fold each, and one database transaction makes the whole batch atomic
// before its WAL checkpoint can be committed.
func (t *Sink) writeMerge(ctx context.Context, tbl *table, b filament.Batch) (filament.WriteReceipt, error) {
	if tbl.upsertSQL == "" {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: merge on keyless resource %q", b.Resource)
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return filament.WriteReceipt{}, copyError("begin merge", b.Resource, b.Seq, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var nbytes int64
	rows := 0
	n := b.NumRows()
	for lo := 0; lo < n; {
		deleting := b.Op(lo) == filament.OpDelete
		hi := lo + 1
		for hi < n && (b.Op(hi) == filament.OpDelete) == deleting {
			hi++
		}
		var (
			k   int
			nb  int64
			err error
		)
		if deleting {
			k, nb, err = t.deleteRange(ctx, tx, tbl, b, lo, hi)
		} else {
			k, nb, err = t.upsertRange(ctx, tx, tbl, b, lo, hi)
		}
		if err != nil {
			return filament.WriteReceipt{}, err
		}
		rows += k
		nbytes += nb
		lo = hi
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.WriteReceipt{}, copyError("commit merge", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes), nil
}

func (t *Sink) receipt(b filament.Batch, rows int, nbytes int64) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:      fmt.Sprintf("postgres://%s.%s", t.schema, b.Resource),
		Bytes:    nbytes,
		Rows:     rows,
		WriteCRC: batch.CRC(b.Rows, b.Ops),
	}
}

// Commit releases the pool; the COPYs are already durable.
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
	// A resumable table keeps its partial load so a later resume can finish it — don't
	// truncate. (The engine also skips Abort on a resumable failure; this guards any
	// other Abort path.)
	cleanup := context.WithoutCancel(ctx)
	for _, tbl := range t.tables {
		if tbl.resumable {
			continue
		}
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
