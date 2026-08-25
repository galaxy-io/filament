//go:build integration

package testcontainers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// PG is an ephemeral Postgres container plus a live pgx pool. Snapshot/Restore
// give per-test isolation WITHOUT re-seeding: seed once, snap, then Restore
// between tests — a filesystem-speed CREATE DATABASE … TEMPLATE, not a
// row-by-row reload.
type PG struct {
	Container    *postgres.PostgresContainer
	dsn          string
	snapshotName string
	pool         *pgxpool.Pool
}

// PGOption configures the Postgres container.
type PGOption func(*pgConfig)

type pgConfig struct {
	database string
	username string
	password string
	image    string
	logical  bool
}

// WithDatabase sets the database name (default: "test").
func WithDatabase(db string) PGOption { return func(c *pgConfig) { c.database = db } }

// WithUsername sets the database user (default: "test").
func WithUsername(u string) PGOption { return func(c *pgConfig) { c.username = u } }

// WithPassword sets the database password (default: "test").
func WithPassword(p string) PGOption { return func(c *pgConfig) { c.password = p } }

// WithImage overrides the container image. By default Postgres reads
// POSTGRES_IMAGE from docker/.env via Image().
func WithImage(img string) PGOption { return func(c *pgConfig) { c.image = img } }

// WithLogicalReplication starts PostgreSQL with wal_level=logical for CDC tests.
func WithLogicalReplication() PGOption { return func(c *pgConfig) { c.logical = true } }

// Postgres starts an exclusive Postgres container, opens a pool, and registers
// cleanup. For the suite-wide instance use SharedPostgres.
// The pool is owned by PG — read it via Pool(); do not cache across Restore.
func Postgres(t testing.TB, opts ...PGOption) *PG {
	t.Helper()
	pg := startPostgres(t, opts...)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(pg.Container) })
	pg.openPool(t)
	return pg
}

// startPostgres boots the container with no test-scoped cleanup and no pool:
// shared instances outlive any one test and are reaped at process exit.
func startPostgres(t testing.TB, opts ...PGOption) *PG {
	t.Helper()
	cfg := &pgConfig{database: "test", username: "test", password: "test"}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.image == "" {
		cfg.image = Image(t, "POSTGRES_IMAGE")
	}

	ctx := context.Background()
	containerOpts := []testcontainers.ContainerCustomizer{
		postgres.WithDatabase(cfg.database),
		postgres.WithUsername(cfg.username),
		postgres.WithPassword(cfg.password),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	}
	if cfg.logical {
		containerOpts = append(containerOpts, testcontainers.WithCmd(
			"postgres", "-c", "fsync=off", "-c", "wal_level=logical", "-c", "max_replication_slots=10", "-c", "max_wal_senders=10",
		))
	}
	ctr, err := postgres.Run(ctx, cfg.image, containerOpts...)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return &PG{
		Container:    ctr,
		dsn:          dsn,
		snapshotName: cfg.database + "_filament_snapshot",
	}
}

// Pool returns the current live pool. Always re-read after Restore.
func (p *PG) Pool() *pgxpool.Pool { return p.pool }

// DSN returns the connection string (sslmode=disable).
func (p *PG) DSN() string { return p.dsn }

// SnapshotCtx marks the current database state as the restore point. Closes
// and reopens the pool around the call — take no pool handle across Snapshot;
// call Pool() again afterward. Safe to call from non-test contexts (e.g. CLI).
func (p *PG) SnapshotCtx(ctx context.Context) error {
	p.pool.Close()
	if err := p.Container.Snapshot(ctx, postgres.WithSnapshotName(p.snapshotName)); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	pool, err := pgxpool.New(ctx, p.dsn)
	if err != nil {
		return fmt.Errorf("open pool after snapshot: %w", err)
	}
	p.pool = pool
	return nil
}

// RestoreCtx resets the database to the last SnapshotCtx. Closes and reopens
// the pool — any handle taken before Restore is dead; call Pool() again.
// Safe to call from non-test contexts (e.g. CLI).
func (p *PG) RestoreCtx(ctx context.Context) error {
	p.pool.Close()
	restoreErr := p.restoreDatabase(ctx)
	pool, err := pgxpool.New(ctx, p.dsn)
	if err != nil {
		return errors.Join(wrapRestoreError(restoreErr), fmt.Errorf("open pool after restore: %w", err))
	}
	p.pool = pool
	if restoreErr != nil {
		return fmt.Errorf("restore: %w", restoreErr)
	}
	return nil
}

// restoreDatabase performs the same template restore as testcontainers, but
// waits for DROP DATABASE to become visible before issuing CREATE DATABASE.
// PostgreSQL can otherwise briefly report success from a forced drop while a
// following create still sees the old database.
func (p *PG) restoreDatabase(ctx context.Context) error {
	cfg, err := pgx.ParseConfig(p.dsn)
	if err != nil {
		return fmt.Errorf("parse postgres dsn: %w", err)
	}
	database := cfg.Database
	cfg.Database = "postgres"

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to postgres admin database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname IN ($1, $2) AND pid <> pg_backend_pid()`, database, p.snapshotName); err != nil {
		return fmt.Errorf("terminate database connections: %w", err)
	}
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)"); err != nil {
		return fmt.Errorf("drop database: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, database).Scan(&exists); err != nil {
			return fmt.Errorf("check dropped database: %w", err)
		}
		if !exists {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database %q still exists after forced drop", database)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}

	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()+" WITH TEMPLATE "+pgx.Identifier{p.snapshotName}.Sanitize()+" OWNER "+pgx.Identifier{cfg.User}.Sanitize())
	if err != nil {
		return fmt.Errorf("create database from snapshot: %w", err)
	}
	return nil
}

func wrapRestoreError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("restore: %w", err)
}

// Snapshot marks the current database state as the restore point. Call once
// after seeding. Closes and reopens the pool around the call — take no pool
// handle across Snapshot; call Pool() again afterward.
func (p *PG) Snapshot(t testing.TB) {
	t.Helper()
	if err := p.SnapshotCtx(context.Background()); err != nil {
		t.Fatalf("%v", err)
	}
	t.Cleanup(p.pool.Close)
}

// Restore resets the database to the last Snapshot. Closes and reopens the
// pool — any handle taken before Restore is dead; call Pool() again.
func (p *PG) Restore(t testing.TB) {
	t.Helper()
	if err := p.RestoreCtx(context.Background()); err != nil {
		t.Fatalf("%v", err)
	}
	t.Cleanup(p.pool.Close)
}

func (p *PG) openPool(t testing.TB) {
	t.Helper()
	if err := p.connect(); err != nil {
		t.Fatalf("%v", err)
	}
	t.Cleanup(p.pool.Close)
}

func (p *PG) connect() error {
	pool, err := pgxpool.New(context.Background(), p.dsn)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	p.pool = pool
	return nil
}

// Wipe returns the database to the pristine snapshot. Replication slots live
// at the cluster level and block the restore's DROP DATABASE, so any left by
// a CDC test are dropped first, along with lingering connections.
func (p *PG) Wipe(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	if err := p.dropReplicationSlots(ctx); err != nil {
		t.Fatalf("wipe: %v", err)
	}
	if _, err := p.pool.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid()`); err != nil {
		t.Fatalf("wipe: terminate connections: %v", err)
	}
	if err := p.RestoreCtx(ctx); err != nil {
		t.Fatalf("wipe: %v", err)
	}
}

func (p *PG) dropReplicationSlots(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		_, err := p.pool.Exec(ctx, `SELECT pg_drop_replication_slot(slot_name) FROM pg_replication_slots`)
		if err == nil {
			return nil
		}
		if attempt >= 4 {
			return fmt.Errorf("drop replication slots: %w", err)
		}
		// An active slot can't be dropped; kill its backend and retry.
		_, _ = p.pool.Exec(ctx, `SELECT pg_terminate_backend(active_pid) FROM pg_replication_slots WHERE active_pid IS NOT NULL`)
		time.Sleep(200 * time.Millisecond)
	}
}
