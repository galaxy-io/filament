package connection

import (
	"context"
	"database/sql"
	"fmt"
)

// Open constructs a database handle from a resolved MySQL driver config. The
// driver establishes the network connection lazily on the first operation.
func Open(_ context.Context, resolved Resolved) (*sql.DB, error) {
	return sql.Open("mysql", resolved.DriverConfig.FormatDSN())
}

// Test opens, pings, and closes a short-lived database handle.
func Test(ctx context.Context, resolved Resolved) error {
	db, err := Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}
