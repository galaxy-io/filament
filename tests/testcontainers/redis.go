//go:build integration || e2e

package testcontainers

import (
	"context"
	"fmt"
	"testing"

	redisgo "github.com/redis/go-redis/v9"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Redis holds an ephemeral Redis container and a connected client.
type Redis struct {
	Container tc.Container
	Addr      string
	Client    *redisgo.Client
}

// RedisContainer starts a Redis container (image from REDIS_IMAGE in docker/.env),
// connects a client, and registers cleanup.
func RedisContainer(t testing.TB) *Redis {
	t.Helper()
	ctx := context.Background()

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ProviderType: providerType(t),
		ContainerRequest: tc.ContainerRequest{
			Image:        Image(t, "REDIS_IMAGE"),
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	cleanupContainer(t, "redis", ctr)

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("redis host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("redis port: %v", err)
	}
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	client := redisgo.NewClient(&redisgo.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	return &Redis{Container: ctr, Addr: addr, Client: client}
}
