package nats

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func TestSubscribeErrorIdentifiesDurableAndRoute(t *testing.T) {
	cause := errors.New("consumer configuration mismatch")
	err := subscribeError("ingestion.v1.run.*.*.requested", "dispatch", "EVENTBUS", cause)

	for _, want := range []string{`durable "dispatch"`, `pattern "ingestion.v1.run.*.*.requested"`, `stream "EVENTBUS"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("subscribe error %q does not contain %q", err, want)
		}
	}
	if !errors.Is(err, cause) {
		t.Fatalf("subscribe error does not wrap cause: %v", err)
	}
}
