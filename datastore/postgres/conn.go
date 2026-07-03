package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver, used only for migrations
)

// NewSQLDB opens a database/sql handle over the "pgx" driver, for running
// migrations (goose needs database/sql, not a pgxpool.Pool).
func NewSQLDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: open: %w", err)
	}
	return db, nil
}

// NewPool opens a pgx connection pool for runtime queries.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("datastore/postgres: ping: %w", err)
	}
	return pool, nil
}
