// Package sqlserver implements filament.Source for Microsoft SQL Server 2017
// and later, and Azure SQL: full snapshots read by primary key.
//
// # Reading
//
// Every query goes through go-mssqldb's RawRows, which hands the source each
// row's TDS value bytes instead of driver.Values. Decoders chosen once per
// result column write those bytes straight into the pipeline's row writers
// (types.go), so a row costs no allocation on the read path. The rest of the
// session — TLS, authentication, parameters, cancellation, Always Encrypted —
// is the driver's.
//
// A table is read by its primary key in shards over the leading key column,
// one streaming query per shard on its own connection (keyset.go). SQL Server
// stores a table in its clustered index, normally the primary key, so each
// shard is a sequential range scan. Each shard runs in a SNAPSHOT
// transaction when the database allows it, else READ COMMITTED.
package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/sqlserver/internal/connection"
)

const (
	defaultSchema = "dbo"

	// defaultShardRows is the target row count per shard. A table at or below
	// it reads as one shard; a larger one splits so it does not hold up a
	// single connection while others sit idle.
	defaultShardRows = 1_000_000
)

// Isolation modes for shard reads.
const (
	isolationAuto          = "auto"
	isolationSnapshot      = "snapshot"
	isolationReadCommitted = "read_committed"
)

// Source reads tables from a SQL Server database. One instance serves one
// run: Configure opens the pool, Extract reads, Teardown closes the pool.
type Source struct {
	profile connection.Profile

	db        *sql.DB
	schema    string
	shardRows int64
	isolation sql.IsolationLevel

	maxConns int
}

// New returns an unconfigured source for a self-managed SQL Server.
func New() *Source { return newSource(connection.SQLServer) }

// NewAzureSQL returns an unconfigured source for Azure SQL Database or Azure
// SQL Managed Instance. It reads exactly as New does and differs only in the
// connection settings it accepts.
func NewAzureSQL() *Source { return newSource(connection.AzureSQL) }

func newSource(p connection.Profile) *Source {
	return &Source{profile: p, schema: defaultSchema, shardRows: defaultShardRows}
}

var (
	_ filament.Source          = (*Source)(nil)
	_ filament.Discoverable    = (*Source)(nil)
	_ filament.LiveValidatable = (*Source)(nil)
	_ filament.SchemaProvider  = (*Source)(nil)
	_ filament.Resumable       = (*Source)(nil)
	_ filament.ResumePlanner   = (*Source)(nil)
)

// Validate checks the connection settings and pipeline options without
// connecting.
func (s *Source) Validate(cfg filament.Config) error {
	if _, err := s.profile.Resolve(cfg); err != nil {
		return fmt.Errorf("sqlserver source: connection config: %w", err)
	}
	switch cfg.String("isolation") {
	case "", isolationAuto, isolationSnapshot, isolationReadCommitted:
	default:
		return fmt.Errorf("sqlserver source: unsupported isolation %q", cfg.String("isolation"))
	}
	if cfg.Has("shard_rows") && cfg.Int("shard_rows") < 0 {
		return fmt.Errorf("sqlserver source: shard_rows must not be negative")
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the server.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := s.profile.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("sqlserver source: connection config: %w", err)
	}
	if err := connection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("sqlserver source: %w", err)
	}
	return nil
}

// Configure opens the pool and settles the isolation level shard reads use.
// Set max_conns to at least the run's parallelism so every shard read gets a
// connection instead of queueing.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := s.profile.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("sqlserver source: connection config: %w", err)
	}
	if v := cfg.String("schema"); v != "" {
		s.schema = v
	}
	if cfg.Has("shard_rows") {
		s.shardRows = int64(cfg.Int("shard_rows")) // 0 disables sharding
	}
	db, err := connection.Open(resolved)
	if err != nil {
		return fmt.Errorf("sqlserver source: open: %w", err)
	}
	if n := cfg.Int("max_conns"); n > 0 {
		s.maxConns = n
		db.SetMaxOpenConns(n)
		db.SetMaxIdleConns(n)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("sqlserver source: ping: %w", err)
	}
	s.db = db
	if s.isolation, err = s.resolveIsolation(ctx, cfg.String("isolation")); err != nil {
		_ = db.Close()
		s.db = nil
		return err
	}
	return nil
}

// resolveIsolation picks SNAPSHOT when the database allows it, unless the
// configuration pins a level. Snapshot isolation reads a consistent point in
// time without shared locks; READ COMMITTED is what a database without it
// offers (row-versioned when READ_COMMITTED_SNAPSHOT is on).
func (s *Source) resolveIsolation(ctx context.Context, mode string) (sql.IsolationLevel, error) {
	if mode == isolationReadCommitted {
		return sql.LevelReadCommitted, nil
	}
	var state int
	const q = `SELECT snapshot_isolation_state FROM sys.databases WHERE database_id = DB_ID()`
	if err := s.db.QueryRowContext(ctx, q).Scan(&state); err != nil {
		return 0, fmt.Errorf("sqlserver source: snapshot isolation state: %w", err)
	}
	enabled := state == 1 // ON
	switch {
	case enabled:
		return sql.LevelSnapshot, nil
	case mode == isolationSnapshot:
		return 0, fmt.Errorf("sqlserver source: isolation is snapshot but the database does not allow it (ALTER DATABASE … SET ALLOW_SNAPSHOT_ISOLATION ON)")
	default:
		return sql.LevelReadCommitted, nil
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
