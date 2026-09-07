//go:build integration || e2e

package testcontainers

import (
	"context"
	"fmt"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NATS holds an ephemeral NATS container and a connected client.
type NATS struct {
	Container tc.Container
	URL       string
	Conn      *natsgo.Conn
}

// NATSContainer starts a NATS container (image from NATS_IMAGE in docker/.env),
// connects a client, and registers cleanup.
func NATSContainer(t testing.TB) *NATS {
	t.Helper()
	ctx := context.Background()

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        Image(t, "NATS_IMAGE"),
			Cmd:          []string{"-js"},
			ExposedPorts: []string{"4222/tcp"},
			// Waiting on the log alone can win a Docker Desktop race where the
			// process is ready but the host-port binding is not inspectable yet.
			WaitingFor: wait.ForListeningPort("4222/tcp").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	cleanupContainer(t, "nats", ctr)

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("nats host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("nats port: %v", err)
	}
	url := fmt.Sprintf("nats://%s:%s", host, port.Port())

	conn, err := natsgo.Connect(url)
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	t.Cleanup(conn.Close)

	return &NATS{Container: ctr, URL: url, Conn: conn}
}
