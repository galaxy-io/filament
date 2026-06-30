//go:build integration

package testcontainers

import (
	"context"
	"fmt"
	"testing"

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
			WaitingFor:   wait.ForLog("Server is ready"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	t.Cleanup(func() { _ = tc.TerminateContainer(ctr) })

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("nats host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "4222")
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
