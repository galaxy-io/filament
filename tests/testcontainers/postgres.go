//go:build integration

package testcontainers

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// PG is an ephemeral Postgres container plus a live pgx pool. Snapshot/Restore
// give per-test isolation WITHOUT re-seeding: seed once, snap, then Restore
// between tests — a filesystem-speed CREATE DATABASE … TEMPLATE, not a
// row-by-row reload.
type PG struct {
	Container *postgres.PostgresContainer
	dsn       string
	pool      *pgxpool.Pool
}

// PGOption configures the Postgres container.
type PGOption func(*pgConfig)

type pgConfig struct {
	database string
	username string
	password string
	image    string
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

// Postgres starts a Postgres container, opens a pool, and registers cleanup.
// The pool is owned by PG — read it via Pool(); do not cache across Restore.
func Postgres(t testing.TB, opts ...PGOption) *PG {
	t.Helper()
	cfg := &pgConfig{database: "test", username: "test", password: "test"}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.image == "" {
		cfg.image = Image(t, "POSTGRES_IMAGE")
	}

	ctx := context.Background()
	ctr, err := postgres.Run(ctx, cfg.image,
		postgres.WithDatabase(cfg.database),
		postgres.WithUsername(cfg.username),
		postgres.WithPassword(cfg.password),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pg := &PG{Container: ctr, dsn: dsn}
	pg.openPool(t)
	return pg
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
	if err := p.Container.Snapshot(ctx); err != nil {
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
	if err := p.Container.Restore(ctx); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	pool, err := pgxpool.New(ctx, p.dsn)
	if err != nil {
		return fmt.Errorf("open pool after restore: %w", err)
	}
	p.pool = pool
	return nil
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
	pool, err := pgxpool.New(context.Background(), p.dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	p.pool = pool
}
