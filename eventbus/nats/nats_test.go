package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestTTLFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", want: defaultTTL},
		{name: "disabled", value: "0"},
		{name: "configured", value: "7200", want: 2 * time.Hour},
		{name: "invalid", value: "forever", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NATS_TTL_SECONDS", tt.value)
			got, err := ttlFromEnv()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ttlFromEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("ttlFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
