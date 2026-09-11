package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/galaxy-io/filament"
	mysqlconnection "github.com/galaxy-io/filament/connectors/mysql/internal/connection"
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
	resolved, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql sink: connection config: %w", err)
	}
	resolved.DriverConfig.DBName = ""
	if err := mysqlconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("mysql sink: %w", err)
	}
	return nil
}

// Open reads dsn/database and opens a pool sized for the run's write parallelism. It
// does no DDL beyond CREATE DATABASE — tables are created per resource by
// EnsureSchema before extraction.
func (t *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql sink: connection config: %w", err)
	}
	mc := resolved.DriverConfig
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
	bdb, err := mysqlconnection.Open(ctx, mysqlconnection.Resolved{DriverConfig: &bootstrap})
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
	db, err := mysqlconnection.Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("mysql sink: open: %w", err)
	}
	if n := run.Options.SnapshotParallelism; n > 0 {
		db.SetMaxOpenConns(max(n, 4))
	}
	t.db = db
	return nil
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
