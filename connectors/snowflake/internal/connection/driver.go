package connection

import (
	"context"
	"database/sql"
	"fmt"

	gosnowflake "github.com/snowflakedb/gosnowflake/v2"
)

// Open returns a database handle backed by the official Snowflake connector.
// The handle does not establish a network connection until its first operation.
func Open(_ context.Context, resolved Resolved) *sql.DB {
	return sql.OpenDB(gosnowflake.NewConnector(gosnowflake.SnowflakeDriver{}, *resolved.DriverConfig))
}

// Test opens, pings, and closes a short-lived Snowflake database handle.
func Test(ctx context.Context, resolved Resolved) error {
	db := Open(ctx, resolved)
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}
