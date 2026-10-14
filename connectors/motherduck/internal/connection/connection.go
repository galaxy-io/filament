package connection

import (
	"context"
	"database/sql/driver"
	"fmt"

	duckdb "github.com/marcboeker/go-duckdb/v2"
)

// Open returns a connector for the resolved database. Every connection it
// hands out runs in UTC so naive timestamps keep their wall clock.
func Open(resolved Resolved) (*duckdb.Connector, error) {
	connector, err := duckdb.NewConnector(resolved.DSN(), func(execer driver.ExecerContext) error {
		_, err := execer.ExecContext(context.Background(), "SET TimeZone='UTC'", nil)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return connector, nil
}

// Test opens one short-lived connection and executes a trivial query.
func Test(ctx context.Context, resolved Resolved) error {
	connector, err := Open(resolved)
	if err != nil {
		return err
	}
	defer func() { _ = connector.Close() }()
	conn, err := connector.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close() }()
	rows, err := conn.(driver.QueryerContext).QueryContext(ctx, "SELECT 1", nil)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	return rows.Close()
}
