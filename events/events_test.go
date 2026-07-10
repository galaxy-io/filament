package events

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
)

func env() Envelope {
	return Envelope{Tenant: "t1", Run: "run1", Resource: "orders", Seq: 7, At: time.Now().UTC()}
}

func TestMarshalRoundTrip(t *testing.T) {
	b, err := Marshal(Fact{Envelope: env(), Name: RunCompleted.Name(), Data: RunCompletedEvent{Records: 934, Bytes: 1024}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "run.completed" || got.Run != "run1" || got.Seq != 7 {
		t.Fatalf("fact = %+v", got)
	}
	payload, ok := got.Data.(RunCompletedEvent)
	if !ok || payload.Records != 934 {
		t.Fatalf("payload = %#v", got.Data)
	}

	// empty payload kind
	b, err = Marshal(Fact{Envelope: env(), Name: RunStarted.Name(), Data: RunStartedEvent{}})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if got, err = Unmarshal(b); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	} else if _, ok := got.Data.(RunStartedEvent); !ok {
		t.Fatalf("empty payload = %#v", got.Data)
	}
}

func TestUnmarshalUnknownType(t *testing.T) {
	if _, err := Unmarshal([]byte(`{"type":"run.exploded","tenant":"t1","run":"run1"}`)); err == nil {
		t.Fatal("want error for unregistered type")
	}
}

func TestCodecRoundTrip(t *testing.T) {
	in := Fact{Envelope: env(), Name: RunFailed.Name(), Data: RunFailedEvent{Error: "boom"}}
	b, err := Codec.Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := Codec.Decode(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	f, ok := out.(Fact)
	if !ok || f.Data.(RunFailedEvent).Error != "boom" {
		t.Fatalf("decoded = %#v", out)
	}
	if _, err := Codec.Encode("not a fact"); err == nil {
		t.Fatal("want error for non-Fact payload")
	}
}

func TestDefineValidatesName(t *testing.T) {
	for _, bad := range []string{"nodot", "a.b.c", ".b", "a."} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("define(%q): want panic", bad)
				}
			}()
			define[struct{}](bad)
		}()
	}

	defer func() {
		if recover() == nil {
			t.Fatal("want panic on duplicate define")
		}
	}()
	define[struct{}]("test.dup")
	define[struct{}]("test.dup")
}

func TestEmitValidatesEnvelope(t *testing.T) {
	bus := inproc.New()
	bad := Envelope{Tenant: "bad.tenant", Run: "run1"}
	if err := Emit(context.Background(), bus, RunStarted, bad, RunStartedEvent{}); err == nil {
		t.Fatal("want error for invalid tenant token")
	}
	if err := Emit(context.Background(), bus, EventType[RunStartedEvent]{}, env(), RunStartedEvent{}); err == nil {
		t.Fatal("want error for zero EventType")
	}
	if _, err := On(context.Background(), bus, EventType[RunStartedEvent]{}, eventbus.SubOpts{}, nil); err == nil {
		t.Fatal("want error for zero EventType in On")
	}
}

func TestEmitOnRoundTrip(t *testing.T) {
	bus := inproc.New()

	ctx := context.Background()

	raw, err := bus.Subscribe("ingestion.v1.>", eventbus.SubOpts{})
	if err != nil {
		t.Fatalf("subscribe raw: %v", err)
	}
	seen := make(chan Event[RunCompletedEvent], 1)
	cancel, err := On(ctx, bus, RunCompleted, eventbus.SubOpts{}, func(_ context.Context, e Event[RunCompletedEvent]) error {
		seen <- e
		return nil
	})
	if err != nil {
		t.Fatalf("on: %v", err)
	}
	defer cancel()

	if err := Emit(context.Background(), bus, RunCompleted, env(), RunCompletedEvent{Records: 3}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	select {
	case e := <-seen:
		if e.Data.Records != 3 || e.Tenant != "t1" {
			t.Fatalf("event = %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("typed handler never fired")
	}
	select {
	case msg := <-raw.C():
		if msg.Subject() != "ingestion.v1.run.t1.run1.completed" {
			t.Fatalf("subject = %q", msg.Subject())
		}
		if _, ok := msg.Payload().(Fact); !ok {
			t.Fatalf("in-proc payload = %T, want Fact by reference", msg.Payload())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no raw delivery")
	}
}

func TestOnCtxCancelClosesSubscription(t *testing.T) {
	bus := inproc.New()

	ctx, cancel := context.WithCancel(context.Background())
	fired := make(chan struct{}, 1)
	_, err := On(ctx, bus, RunCompleted, eventbus.SubOpts{}, func(context.Context, Event[RunCompletedEvent]) error {
		fired <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("on: %v", err)
	}

	cancel()
	time.Sleep(50 * time.Millisecond) // let the watcher close the sub

	if err := Emit(context.Background(), bus, RunCompleted, env(), RunCompletedEvent{}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	select {
	case <-fired:
		t.Fatal("handler fired after ctx cancel")
	case <-time.After(200 * time.Millisecond):
	}
}

type fakeMsg struct{ f Fact }

func (m fakeMsg) Subject() string { return "" }
func (m fakeMsg) Payload() any    { return m.f }
func (m fakeMsg) Seq() uint64     { return 1 }
func (m fakeMsg) Ack() error      { return nil }
func (m fakeMsg) Nak() error      { return nil }

func TestMuxDispatch(t *testing.T) {
	mux := NewMux()
	var typed, anyName string
	Handle(mux, RunFailed, func(_ context.Context, e Event[RunFailedEvent]) error {
		typed = e.Data.Error
		return errors.New("boom")
	})
	mux.HandleAny(func(_ context.Context, f Fact) error {
		anyName = f.Name
		return nil
	})

	f := Fact{Envelope: env(), Name: RunFailed.Name(), Data: RunFailedEvent{Error: "sink exploded"}}
	err := mux.Dispatch(context.Background(), fakeMsg{f: f})

	if typed != "sink exploded" || anyName != "run.failed" {
		t.Fatalf("typed=%q any=%q", typed, anyName)
	}
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want joined handler error", err)
	}
}
