package connection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open constructs a PostgreSQL pool from a resolved driver config.
func Open(ctx context.Context, resolved Resolved) (*pgxpool.Pool, error) {
	return pgxpool.NewWithConfig(ctx, resolved.DriverConfig)
}

// Test opens, pings, and closes a short-lived PostgreSQL pool.
func Test(ctx context.Context, resolved Resolved) error {
	pool, err := Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}
