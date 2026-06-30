package inproc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/galaxy-io/filament/eventbus"
)

// payload is a stand-in domain event; the bus treats it as opaque.
type payload struct {
	kind string
	n    int
}

func recv(t *testing.T, sub eventbus.Subscription) eventbus.Message {
	t.Helper()
	select {
	case m := <-sub.C():
		return m
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func mustNotRecv(t *testing.T, sub eventbus.Subscription) {
	t.Helper()
	select {
	case m := <-sub.C():
		t.Fatalf("unexpected delivery: %s", m.Subject())
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPublishDeliver(t *testing.T) {
	b := New()
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	sub, err := b.Subscribe("app.v1.run.t1.r1.*", eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(ctx, "app.v1.run.t1.r1.started", payload{"started", 1}); err != nil {
		t.Fatal(err)
	}

	m := recv(t, sub)
	if m.Subject() != "app.v1.run.t1.r1.started" {
		t.Errorf("subject = %q", m.Subject())
	}
	if m.Seq() != 1 {
		t.Errorf("stream seq = %d, want 1", m.Seq())
	}
	got, ok := m.Payload().(payload)
	if !ok || got.kind != "started" {
		t.Errorf("payload = %#v", m.Payload())
	}
	if err := m.Ack(); err != nil {
		t.Errorf("ack: %v", err)
	}
}

func TestRoutingFanOut(t *testing.T) {
	b := New()
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	all, _ := b.Subscribe("app.v1.>", eventbus.SubOpts{})           // firehose
	mine, _ := b.Subscribe("app.v1.*.t1.r1.*", eventbus.SubOpts{})  // my run
	other, _ := b.Subscribe("app.v1.*.t1.r9.*", eventbus.SubOpts{}) // different run

	subj := "app.v1.batch.t1.r1.written"
	if err := b.Publish(ctx, subj, payload{"written", 1}); err != nil {
		t.Fatal(err)
	}

	if recv(t, all).Subject() != subj {
		t.Error("firehose missed the event")
	}
	if recv(t, mine).Subject() != subj {
		t.Error("run subscriber missed the event")
	}
	mustNotRecv(t, other) // wrong run — must not match
}

func TestPublishRejectsEmptySubject(t *testing.T) {
	b := New()
	defer func() { _ = b.Close() }()

	err := b.Publish(context.Background(), "", payload{})
	if !errors.Is(err, eventbus.ErrInvalidToken) {
		t.Errorf("Publish(empty subject) = %v, want ErrInvalidToken", err)
	}
}

func TestReplay(t *testing.T) {
	b := New()
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		if err := b.Publish(ctx, "app.v1.resource.t1.r1.page_fetched", payload{"page", i}); err != nil {
			t.Fatal(err)
		}
	}

	// late subscriber replays the whole backlog, in order.
	sub, err := b.Replay(ctx, "app.v1.>", 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(1); i <= 3; i++ {
		if got := recv(t, sub).Seq(); got != i {
			t.Errorf("replay seq = %d, want %d", got, i)
		}
	}
}

func TestNakRedelivers(t *testing.T) {
	b := New(WithMaxDeliveries(3))
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	sub, err := b.Subscribe("app.v1.>", eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(ctx, "app.v1.run.t1.r1.started", payload{"started", 1}); err != nil {
		t.Fatal(err)
	}

	// Nak twice, then receive a third time (tries: 1→2→3, capped at 3).
	m := recv(t, sub)
	if err := m.Nak(); err != nil {
		t.Fatal(err)
	}
	m = recv(t, sub)
	if err := m.Nak(); err != nil {
		t.Fatal(err)
	}
	m = recv(t, sub)
	if err := m.Nak(); err != nil { // at the cap — dropped, no further delivery
		t.Fatal(err)
	}
	mustNotRecv(t, sub)
}

// in-proc's Resolve is client-side: no native target, all matching on ClientFilter.
func TestResolveClientSide(t *testing.T) {
	b := New()
	defer func() { _ = b.Close() }()

	r, err := b.Resolve("some.message.*.type.>")
	if err != nil {
		t.Fatal(err)
	}
	if r.Target != "" || r.Exact {
		t.Errorf("route = (Target %q, Exact %v), want empty,false", r.Target, r.Exact)
	}
	if !r.ClientFilter.MatchSubject("some.message.A.type.x") {
		t.Error("ClientFilter did not match a valid subject")
	}
	if _, err := b.Resolve("a.>.b"); !errors.Is(err, eventbus.ErrBadPattern) {
		t.Errorf("Resolve(bad) = %v, want ErrBadPattern", err)
	}
}

func TestClosedBusRejects(t *testing.T) {
	b := New()
	_ = b.Close()

	if err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", payload{}); !errors.Is(err, eventbus.ErrBusClosed) {
		t.Errorf("Publish after close = %v, want ErrBusClosed", err)
	}
	if _, err := b.Subscribe("app.v1.>", eventbus.SubOpts{}); !errors.Is(err, eventbus.ErrBusClosed) {
		t.Errorf("Subscribe after close = %v, want ErrBusClosed", err)
	}
}
