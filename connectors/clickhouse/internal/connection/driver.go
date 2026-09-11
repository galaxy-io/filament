package connection

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Open constructs a ClickHouse connection from resolved driver options.
func Open(_ context.Context, resolved Resolved) (chdriver.Conn, error) {
	return ch.Open(resolved.DriverConfig)
}

// Test opens, pings, and closes a short-lived ClickHouse connection.
func Test(ctx context.Context, resolved Resolved) error {
	opts := *resolved.DriverConfig
	opts.Auth.Database = DefaultDatabase
	conn, err := Open(ctx, Resolved{DriverConfig: &opts})
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping over %s to %s: %w", opts.Protocol, opts.Addr[0], err)
	}
	return nil
}
