//go:build integration

package nats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

type strCodec struct{}

func (strCodec) Encode(payload any) ([]byte, error) {
	s, ok := payload.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return []byte(s), nil
}

func (strCodec) Decode(data []byte) (any, error) { return string(data), nil }

// TestDurableConsumerLifecycle proves the three durable-consumer guarantees:
// Close does not delete the server-side consumer, a rebind resumes from the
// kept position, and a consumer deleted out from under a live subscription is
// recreated by the pump.
func TestDurableConsumerLifecycle(t *testing.T) {
	nc := testcontainers.NATSContainer(t)
	const stream = "EVENTBUS_IT"

	bus, err := natsbus.New(nc.URL, strCodec{},
		natsbus.WithStream(stream),
		natsbus.WithSubjects("it.v1.>"),
		natsbus.WithLogf(t.Logf),
	)
	if err != nil {
		t.Fatalf("bus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

	ctx := context.Background()
	pub := func(msg string) {
		t.Helper()
		if err := bus.Publish(ctx, "it.v1.a", msg); err != nil {
			t.Fatalf("publish %s: %v", msg, err)
		}
	}
	recv := func(sub eventbus.Subscription) string {
		t.Helper()
		select {
		case m, ok := <-sub.C():
			if !ok {
				t.Fatal("subscription channel closed")
			}
			_ = m.Ack()
			return m.Payload().(string)
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for message")
			return ""
		}
	}

	// Backlog published before the durable exists; Replay delivers it on creation.
	pub("m1")
	pub("m2")
	sub, err := bus.Subscribe("it.v1.>", eventbus.SubOpts{Durable: "trk", Replay: true})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if got := recv(sub); got != "m1" {
		t.Fatalf("replay: got %q, want m1", got)
	}
	if got := recv(sub); got != "m2" {
		t.Fatalf("replay: got %q, want m2", got)
	}
	_ = sub.Close()

	// Close must leave the durable on the server.
	js, err := nc.Conn.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	if _, err := js.ConsumerInfo(stream, "trk"); err != nil {
		t.Fatalf("durable deleted by Close: %v", err)
	}

	// A rebind resumes from the kept position: only m3 is delivered.
	pub("m3")
	sub2, err := bus.Subscribe("it.v1.>", eventbus.SubOpts{Durable: "trk", Replay: true})
	if err != nil {
		t.Fatalf("resubscribe: %v", err)
	}
	if got := recv(sub2); got != "m3" {
		t.Fatalf("resume: got %q, want m3", got)
	}

	// Server-side deletion mid-flight: the pump recreates the consumer and the
	// replayed backlog (ack state is gone) eventually includes the new message.
	if err := js.DeleteConsumer(stream, "trk"); err != nil {
		t.Fatalf("delete consumer: %v", err)
	}
	pub("m4")
	deadline := time.Now().Add(20 * time.Second)
	for {
		if got := recv(sub2); got == "m4" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("recreated consumer never delivered m4")
		}
	}
	_ = sub2.Close()
}
