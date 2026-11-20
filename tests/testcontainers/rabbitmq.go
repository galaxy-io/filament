//go:build integration || e2e

package testcontainers

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/galaxy-io/filament/tests/testcontainers/internal/envfile"
)

// RabbitMQContainer starts RabbitMQ with the stream plugin and returns a Stream
// URI reachable from the host. The broker advertises the mapped host port so
// producer connections are not sent to the container-internal address.
func RabbitMQContainer(t testing.TB) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	const name = "rabbitmq"
	image := rabbitmqImage(t)
	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ProviderType: providerType(t), ContainerRequest: tc.ContainerRequest{
		Image: image, ExposedPorts: []string{"5552/tcp"},
		Entrypoint: []string{"/bin/sh", "-c", "while [ ! -f /tmp/filament-start.ready ]; do sleep 0.1; done; exec /bin/sh /tmp/filament-start.sh"},
		WaitingFor: wait.ForExec([]string{"/bin/sh", "-c", "test -d /tmp"}).WithStartupTimeout(time.Minute),
	}, Started: true})
	if ctr != nil {
		cleanupContainer(t, name, ctr)
	}
	if err != nil {
		t.Fatalf("start %s (image %s): %v", name, image, err)
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "5552/tcp")
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`#!/bin/sh
set -e
rabbitmq-plugins enable --offline rabbitmq_stream
mkdir -p /etc/rabbitmq/conf.d
cat > /etc/rabbitmq/conf.d/99-filament.conf <<EOF
stream.advertised_host = %s
stream.advertised_port = %s
loopback_users.guest = false
EOF
exec docker-entrypoint.sh rabbitmq-server
`, host, port.Port())
	if err = ctr.CopyToContainer(ctx, []byte(script), "/tmp/filament-start.sh", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ctr.CopyToContainer(ctx, []byte("ready"), "/tmp/filament-start.ready", 0o644); err != nil {
		t.Fatal(err)
	}
	uri := fmt.Sprintf("rabbitmq-stream://guest:guest@%s/%s", net.JoinHostPort(host, port.Port()), "%2f")
	ready, stop := context.WithTimeout(ctx, 2*time.Minute)
	defer stop()
	var dialErr error
	for {
		env, err := stream.NewEnvironment(stream.NewEnvironmentOptions().SetUri(uri))
		if env != nil {
			_ = env.Close()
		}
		if err == nil {
			dialErr = nil
			break
		}
		dialErr = err
		if ready.Err() != nil {
			t.Fatalf("%s readiness: %v", name, dialErr)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return uri
}

func rabbitmqImage(t testing.TB) string {
	t.Helper()
	images, err := envfile.Images()
	if err != nil {
		t.Fatalf("load container images: %v", err)
	}
	image := images["RABBITMQ_IMAGE"]
	if image == "" {
		t.Fatal(`no image for "RABBITMQ_IMAGE"`)
	}
	return image
}
