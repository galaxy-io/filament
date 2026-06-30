//go:build integration

package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	_ "github.com/trinodb/trino-go-client/trino" // registers the "trino" sql driver
)

// Trino holds an ephemeral Trino container and an open *sql.DB. The container
// uses the built-in memory connector, so no external metastore is required.
type Trino struct {
	Container tc.Container
	Endpoint  string // host:port of the HTTP coordinator
	DSN       string // http://test@host:port?catalog=memory
	DB        *sql.DB
}

// TrinoContainer starts a Trino container (image from TRINO_IMAGE in docker/.env),
// opens a connection against the memory catalog, and registers cleanup. Trino is
// slow to boot, so the wait polls /v1/info until the server reports started.
func TrinoContainer(t testing.TB) *Trino {
	t.Helper()
	ctx := context.Background()

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        Image(t, "TRINO_IMAGE"),
			ExposedPorts: []string{"8080/tcp"},
			WaitingFor: wait.ForHTTP("/v1/info").WithPort("8080/tcp").
				WithResponseMatcher(func(body io.Reader) bool {
					b, _ := io.ReadAll(body)
					return strings.Contains(string(b), `"starting":false`)
				}).WithStartupTimeout(3 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start trino container: %v", err)
	}
	t.Cleanup(func() { _ = tc.TerminateContainer(ctr) })

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("trino host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("trino port: %v", err)
	}
	endpoint := fmt.Sprintf("%s:%s", host, port.Port())
	dsn := fmt.Sprintf("http://test@%s?catalog=memory&schema=default", endpoint)

	db, err := sql.Open("trino", dsn)
	if err != nil {
		t.Fatalf("open trino: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("trino ping: %v", err)
	}

	return &Trino{Container: ctr, Endpoint: endpoint, DSN: dsn, DB: db}
}
