//go:build integration || e2e

package testcontainers

import (
	"context"
	"testing"

	tc "github.com/testcontainers/testcontainers-go"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

// CH is an ephemeral ClickHouse server and its host-reachable native DSN.
type CH struct {
	Container *tcclickhouse.ClickHouseContainer
	DSN       string
}

// ClickHouse starts a pinned ClickHouse LTS container and registers cleanup.
func ClickHouse(t testing.TB) *CH {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcclickhouse.Run(ctx, Image(t, "CLICKHOUSE_IMAGE"), tc.WithProvider(providerType(t)))
	if err != nil {
		t.Fatalf("start clickhouse container: %v", err)
	}
	t.Cleanup(func() { _ = tc.TerminateContainer(ctr) })

	dsn, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("clickhouse connection string: %v", err)
	}
	return &CH{Container: ctr, DSN: dsn}
}
