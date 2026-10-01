//go:build integration || e2e

package testcontainers

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/twmb/franz-go/pkg/kgo"
)

// KafkaContainer starts either Kafka or Redpanda on a random mapped port.
// The delayed entrypoint installs the host's advertised address before startup.
func KafkaContainer(t testing.TB, redpanda bool) []string {
	t.Helper()
	ctx := context.Background()
	imageKey, name := "KAFKA_IMAGE", "kafka"
	if redpanda {
		imageKey, name = "REDPANDA_IMAGE", "redpanda"
	}
	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ProviderType: providerType(t), ContainerRequest: tc.ContainerRequest{
		Image: Image(t, imageKey), ExposedPorts: []string{"9092/tcp"},
		Entrypoint: []string{"/bin/sh", "-c", "while [ ! -f /tmp/filament-start.ready ]; do sleep 0.1; done; exec /bin/sh /tmp/filament-start.sh"},
		WaitingFor: wait.ForExec([]string{"/bin/sh", "-c", "test -d /tmp"}).WithStartupTimeout(time.Minute),
	}, Started: true})
	if err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	cleanupContainer(t, name, ctr)
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "9092/tcp")
	if err != nil {
		t.Fatal(err)
	}
	address := net.JoinHostPort(host, port.Port())
	script := fmt.Sprintf(`#!/bin/sh
export KAFKA_NODE_ID=1
export KAFKA_PROCESS_ROLES=broker,controller
export KAFKA_LISTENERS=INTERNAL://:19092,EXTERNAL://:9092,CONTROLLER://:9093
export KAFKA_ADVERTISED_LISTENERS=INTERNAL://localhost:19092,EXTERNAL://%s
export KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
export KAFKA_INTER_BROKER_LISTENER_NAME=INTERNAL
export KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER
export KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093
export KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1
export KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1
export KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1
export KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0
export KAFKA_AUTO_CREATE_TOPICS_ENABLE=false
exec /etc/kafka/docker/run
`, address)
	if redpanda {
		script = fmt.Sprintf("#!/bin/sh\nexec /usr/bin/rpk redpanda start --mode dev-container --smp 1 --memory 512M --reserve-memory 0M --overprovisioned --node-id 0 --check=false --kafka-addr 0.0.0.0:9092 --advertise-kafka-addr %s --set redpanda.auto_create_topics_enabled=false\n", address)
	}
	if err = ctr.CopyToContainer(ctx, []byte(script), "/tmp/filament-start.sh", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ctr.CopyToContainer(ctx, []byte("ready"), "/tmp/filament-start.ready", 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := kgo.NewClient(kgo.SeedBrokers(address))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ready, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		attempt, stop := context.WithTimeout(ready, time.Second)
		err = c.Ping(attempt)
		stop()
		if err == nil {
			break
		}
		if ready.Err() != nil {
			t.Fatalf("%s readiness: %v", name, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return []string{address}
}
