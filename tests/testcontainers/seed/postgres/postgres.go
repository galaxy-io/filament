// Package postgres seeds a Postgres database with the canonical Row schema.
// Generation runs server-side via generate_series — 100k rows is seconds, not
// a million client round-trips. The same Spec always produces byte-identical
// data; Checksum is a salted, order-independent hash so tests can assert
// exact integrity without re-reading every row.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament/tests/testcontainers/seed"
)

// Seeder returns a SeederFunc that opens its own pool against the target DSN
// and calls Apply. Use it when registering a Scenario.
func Seeder(spec seed.Spec) seed.SeederFunc {
	return func(ctx context.Context, dsn string) (seed.Manifest, error) {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return seed.Manifest{}, fmt.Errorf("connect: %w", err)
		}
		defer pool.Close()
		return Apply(ctx, pool, spec)
	}
}

// Dropper returns a DropFunc that removes all seeded tables for the spec.
func Dropper(spec seed.Spec) seed.DropFunc {
	return func(ctx context.Context, dsn string) error {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer pool.Close()
		return Drop(ctx, pool, spec)
	}
}

// Verifier returns a VerifyFunc that re-fingerprints the seeded tables.
func Verifier(spec seed.Spec) seed.VerifyFunc {
	return func(ctx context.Context, dsn string) (seed.Manifest, error) {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return seed.Manifest{}, fmt.Errorf("connect: %w", err)
		}
		defer pool.Close()
		return Verify(ctx, pool, spec)
	}
}

// Apply (re)creates and fills the Spec's tables and returns their manifest.
// Idempotent: each table is dropped and recreated, so re-running with the
// same Spec reproduces identical data and checksums.
func Apply(ctx context.Context, pool *pgxpool.Pool, spec seed.Spec) (seed.Manifest, error) {
	m := seed.Manifest{Spec: spec, Tables: make([]seed.Table, 0, spec.Tables)}
	for i := range spec.Tables {
		name := seed.TableName(i)
		salt := spec.Salt + int64(i)
		if err := createTable(ctx, pool, name); err != nil {
			return seed.Manifest{}, err
		}
		if err := fillTable(ctx, pool, name, salt, spec.RowsPerTable); err != nil {
			return seed.Manifest{}, err
		}
		tbl, err := fingerprint(ctx, pool, name)
		if err != nil {
			return seed.Manifest{}, err
		}
		m.Tables = append(m.Tables, tbl)
		seed.Progressf(ctx, "  %-12s  %d rows\n", name, tbl.Rows)
	}
	return m, nil
}

// Drop removes all tables created by Apply for the given spec.
func Drop(ctx context.Context, pool *pgxpool.Pool, spec seed.Spec) error {
	for i := range spec.Tables {
		name := seed.TableName(i)
		// Names are internal constants — safe to interpolate.
		if _, err := pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %q", name)); err != nil {
			return fmt.Errorf("drop %s: %w", name, err)
		}
		seed.Progressf(ctx, "  dropped %s\n", name)
	}
	return nil
}

// Verify re-fingerprints the seeded tables and returns their current manifest.
func Verify(ctx context.Context, pool *pgxpool.Pool, spec seed.Spec) (seed.Manifest, error) {
	m := seed.Manifest{Spec: spec, Tables: make([]seed.Table, 0, spec.Tables)}
	for i := range spec.Tables {
		name := seed.TableName(i)
		tbl, err := fingerprint(ctx, pool, name)
		if err != nil {
			return seed.Manifest{}, fmt.Errorf("verify %s: %w", name, err)
		}
		m.Tables = append(m.Tables, tbl)
		seed.Progressf(ctx, "  %-12s  %d rows  checksum=%s\n", name, tbl.Rows, tbl.Checksum)
	}
	return m, nil
}

func createTable(ctx context.Context, pool *pgxpool.Pool, name string) error {
	// Names are internal constants (seed_NN), not user input — safe to interpolate.
	ddl := fmt.Sprintf(`
		DROP TABLE IF EXISTS %[1]q;
		CREATE TABLE %[1]q (
			id         uuid        PRIMARY KEY,
			tenant     text        NOT NULL,
			seq        bigint      NOT NULL,
			amount     integer     NOT NULL,
			payload    jsonb       NOT NULL,
			updated_at timestamptz NOT NULL
		)`, name)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	return nil
}

func fillTable(ctx context.Context, pool *pgxpool.Pool, name string, salt int64, rows int) error {
	// Every column is a pure function of the series index g and salt — no
	// random() — so the table is byte-identical on every run.
	ins := fmt.Sprintf(`
		INSERT INTO %q (id, tenant, seq, amount, payload, updated_at)
		SELECT
			md5((g + $1)::text)::uuid,
			't' || (g %% 4),
			g,
			(abs(hashtextextended((g + $1)::text, 0)) %% 1000)::int,
			jsonb_build_object('n', g, 'h', md5((g + $1)::text)),
			timestamptz '2024-01-01 00:00:00Z' + g * interval '1 second'
		FROM generate_series(1, $2::bigint) g`, name)
	if _, err := pool.Exec(ctx, ins, salt, rows); err != nil {
		return fmt.Errorf("fill %s: %w", name, err)
	}
	return nil
}

func fingerprint(ctx context.Context, pool *pgxpool.Pool, name string) (seed.Table, error) {
	// sum() over per-row hashes is order-independent and overflow-safe (numeric).
	q := fmt.Sprintf(
		`SELECT count(*), coalesce(sum(hashtextextended(%[1]q::text, 0)), 0)::text FROM %[1]q`, name)
	var t seed.Table
	t.Name = name
	if err := pool.QueryRow(ctx, q).Scan(&t.Rows, &t.Checksum); err != nil {
		return seed.Table{}, fmt.Errorf("fingerprint %s: %w", name, err)
	}
	return t, nil
}
