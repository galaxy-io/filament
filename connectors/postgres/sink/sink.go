package postgres

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	pgconnection "github.com/galaxy-io/filament/connectors/postgres/internal/connection"
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

const defaultSchema = "public"

// New returns an unconfigured sink. Open wires it to the database.
func New() *Sink { return &Sink{schema: defaultSchema} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (t *Sink) Name() string { return "postgres" }

// Validate checks connection syntax without opening a network connection.
func (t *Sink) Validate(cfg filament.Config) error {
	if _, err := pgconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("postgres sink: connection config: %w", err)
	}
	return nil
}

// TestConnection opens a short-lived pool and verifies the configured
// credentials without creating the destination schema.
func (t *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := pgconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("postgres sink: connection config: %w", err)
	}
	if err := pgconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("postgres sink: %w", err)
	}
	return nil
}

// Open reads dsn/schema and opens a pool sized for the run's write parallelism. It
// does no DDL — tables are created per resource by EnsureSchema before extraction.
func (t *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := pgconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("postgres sink: connection config: %w", err)
	}
	t.dsn = resolved.DSN
	if v := cfg.String("schema"); v != "" {
		t.schema = v
	}
	t.run = run.Run
	t.policies = run.WritePolicies
	t.written.Store(0)
	t.tables = map[string]*table{}

	poolCfg := resolved.DriverConfig
	if n := run.Options.SnapshotParallelism; n > 0 && n <= math.MaxInt32 {
		poolCfg.MaxConns = max(poolCfg.MaxConns, int32(n))
	}
	pool, err := pgconnection.Open(ctx, resolved)
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
