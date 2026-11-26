package connection

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

// Open constructs a database handle from a resolved MySQL driver config. The
// driver establishes the network connection lazily on the first operation.
func Open(_ context.Context, resolved Resolved) (*sql.DB, error) {
	if resolved.DriverConfig.DialFunc == nil {
		return sql.Open("mysql", resolved.DriverConfig.FormatDSN())
	}
	// A DSN cannot carry the tunnel's dial function.
	connector, err := mysql.NewConnector(resolved.DriverConfig)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

// Test opens, pings, and closes a short-lived database handle.
func Test(ctx context.Context, resolved Resolved) error {
	defer func() { _ = resolved.Tunnel.Close() }()
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
