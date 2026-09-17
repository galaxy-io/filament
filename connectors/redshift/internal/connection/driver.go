package connection

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates and connects a Redshift-compatible pgx pool.
func Open(ctx context.Context, resolved Resolved) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, resolved.DriverConfig)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Test opens, pings, and closes a short-lived Redshift connection pool.
func Test(ctx context.Context, resolved Resolved) error {
	pool, err := Open(ctx, resolved)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			endpoint := net.JoinHostPort(resolved.DriverConfig.ConnConfig.Host, strconv.Itoa(int(resolved.DriverConfig.ConnConfig.Port)))
			return fmt.Errorf("connect to Redshift endpoint %s timed out; verify the endpoint, port, VPC route, public-access setting, and security-group inbound rule: %w", endpoint, err)
		}
		return fmt.Errorf("ping: %w", err)
	}
	pool.Close()
	return nil
}
