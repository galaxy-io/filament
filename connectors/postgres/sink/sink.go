package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	pgconnection "github.com/galaxy-io/filament/connectors/postgres/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

// Sink loads each resource into its own typed table with native columns. The engine
// calls EnsureSchema(resource, schema) up front — Sink builds one typed table per
// resource — then each batch is COPYed straight into it (replace, append) or
// through a per-connection temp table into an upsert or delete (upsert, merge). It
// implements filament.Schematized; the engine only runs schema discovery for sinks
// that do.
type Sink struct {
	evolution           schema.Policy
	transactionalWrites bool
	writeMu             sync.Mutex
	session             writeSession

	pool     *pgxpool.Pool
	run      filament.RunID
	schema   string
	dsn      string
	policies map[string]filament.WritePolicy
	written  atomic.Int64

	// Bounded runs prepare tables before extraction. Continuous runs may also
	// prepare inferred schemas on their single writer inside the active epoch.
	tablesMu sync.RWMutex
	tables   map[string]*table
}

// table is one resource's ensured destination: its identifiers, column types, the
// COPY renderers, and the prebuilt statements of the key-based paths. resumable
// marks a table whose writes are idempotent by key, so a resume must preserve its
// rows.
type table struct {
	input     *arrow.Schema
	model     rowmodel.Schema
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
	if _, err := schemaPolicy(cfg.String("schema_evolution")); err != nil {
		return err
	}
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
	t.transactionalWrites = false
	for _, policy := range run.WritePolicies {
		if policy.Capability.Durability == filament.DurabilityAfterCommit {
			t.transactionalWrites = true
		}
	}

	if t.transactionalWrites {
		for _, policy := range run.WritePolicies {
			if policy.Capability.Durability != filament.DurabilityAfterCommit || (policy.Capability.Mode != filament.WriteAppend && policy.Capability.Mode != filament.WriteMerge && policy.Capability.Mode != filament.WriteUpsert) {
				return fmt.Errorf("postgres sink: commit-durable sessions require append, upsert, or merge policies for every resource")
			}
		}
	}
	if err := t.open(ctx, run); err != nil {
		return err
	}
	t.session.lifecycle.Open()
	return nil
}

func (t *Sink) open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	policy, err := schemaPolicy(cfg.String("schema_evolution"))
	if err != nil {
		return err
	}
	t.evolution = policy
	if policy != "" && cfg.String("schema") != "" && !validIdentifier(cfg.String("schema")) {
		return fmt.Errorf("invalid PostgreSQL schema name")
	}
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

// abortSnapshot clears only non-resumable replacement tables.
// Transactional writes instead roll back only their active transaction.
func (t *Sink) abortSnapshot(ctx context.Context) error {
	if t.pool == nil {
		return nil
	}
	// A resumable table keeps its partial load so a later resume can finish it — don't
	// truncate. (The engine also skips Abort on a resumable failure; this guards any
	// other Abort path.)
	var cleanupErr error
	for resource, tbl := range t.tables {
		if t.modeFor(resource) != filament.WriteReplace || tbl.resumable {
			continue
		}
		if _, err := t.pool.Exec(ctx, "TRUNCATE "+tbl.qualified); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("truncate %s: %w", resource, err))
		}
	}
	return cleanupErr
}

func (t *Sink) release() {
	if t.pool != nil {
		t.pool.Close()
		t.pool = nil
	}
}

func (t *Sink) tableFor(resource string) *table {
	t.tablesMu.RLock()
	defer t.tablesMu.RUnlock()
	return t.tables[resource]
}
