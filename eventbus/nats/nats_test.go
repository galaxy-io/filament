package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament/eventbus"
)

type stringCodec struct{}

func (stringCodec) Encode(payload any) ([]byte, error) {
	s, ok := payload.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return []byte(s), nil
}

func TestDecodeFailureIsLoggedWithoutPayload(t *testing.T) {
	var line string
	b := &Bus{logf: func(format string, args ...any) { line = fmt.Sprintf(format, args...) }}
	b.logDecodeFailure("app.v1.run.t1.r1.started", errors.New("bad frame"))
	if !strings.Contains(line, "app.v1.run.t1.r1.started") || !strings.Contains(line, "bad frame") || !strings.Contains(line, "terminating") {
		t.Fatalf("decode log = %q", line)
	}
}

func (stringCodec) Decode(data []byte) (any, error) {
	return string(data), nil
}

func TestResolvePassthrough(t *testing.T) {
	b := &Bus{}
	route, err := b.Resolve("app.v1.run.*.>")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if route.Target != "app.v1.run.*.>" {
		t.Fatalf("target = %q", route.Target)
	}
	if !route.Exact {
		t.Fatal("route should be exact for NATS")
	}
	if !route.ClientFilter.MatchSubject("app.v1.run.t1.r1.started") {
		t.Fatal("client filter should match the resolved pattern")
	}
}

func TestPublishRejectsInvalidSubjectBeforeEncoding(t *testing.T) {
	b := &Bus{codec: stringCodec{}}
	err := b.Publish(context.Background(), "app.v1.*", "payload")
	if !errors.Is(err, eventbus.ErrInvalidToken) {
		t.Fatalf("Publish error = %v, want ErrInvalidToken", err)
	}
}

func TestPublishClosed(t *testing.T) {
	b := &Bus{codec: stringCodec{}}
	b.closed.Store(true)
	err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", "payload")
	if !errors.Is(err, eventbus.ErrBusClosed) {
		t.Fatalf("Publish error = %v, want ErrBusClosed", err)
	}
}

func TestConsumerGoneIncludesDeleteRaceNoResponders(t *testing.T) {
	for _, err := range []error{
		jetstream.ErrConsumerNotFound,
		jetstream.ErrConsumerDeleted,
		natsgo.ErrNoResponders,
		fmt.Errorf("fetch: %w", natsgo.ErrNoResponders),
	} {
		if !consumerGone(err) {
			t.Errorf("consumerGone(%v) = false, want true", err)
		}
	}
	if consumerGone(errors.New("timeout")) {
		t.Fatal("ordinary fetch timeout must not recreate the durable")
	}
}

func TestStreamTTLIsCreatedAndReconciled(t *testing.T) {
	ns, err := natsserver.NewServer(&natsserver.Options{
		DontListen: true,
		JetStream:  true,
		StoreDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("nats server not ready")
	}

	nc, err := natsgo.Connect("", natsgo.InProcessServer(ns))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)

	const streamName = "TTL_TEST"
	bus, err := New("", stringCodec{},
		WithConn(nc),
		WithStream(streamName),
		WithSubjects("ttl.test.>"),
	)
	if err != nil {
		t.Fatalf("create bus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	assertStreamTTL(t, bus, streamName, defaultTTL)

	reconciled, err := New("", stringCodec{},
		WithConn(nc),
		WithStream(streamName),
		WithSubjects("ttl.test.>"),
		WithTTL(2*time.Hour),
	)
	if err != nil {
		t.Fatalf("reconcile bus: %v", err)
	}
	t.Cleanup(func() { _ = reconciled.Close() })
	assertStreamTTL(t, reconciled, streamName, 2*time.Hour)
}

func assertStreamTTL(t *testing.T, bus *Bus, streamName string, want time.Duration) {
	t.Helper()
	stream, err := bus.js.Stream(context.Background(), streamName)
	if err != nil {
		t.Fatalf("load stream: %v", err)
	}
	if got := stream.CachedInfo().Config.MaxAge; got != want {
		t.Fatalf("stream MaxAge = %v, want %v", got, want)
	}
}

func TestNegativeTTLIsRejected(t *testing.T) {
	if _, err := New("", stringCodec{}, WithTTL(-time.Second)); err == nil {
		t.Fatal("New accepted a negative TTL")
	}
}
