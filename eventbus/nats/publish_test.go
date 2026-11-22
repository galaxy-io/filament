package nats

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// flakyJetStream fails the first n publishes with err, then acks.
type flakyJetStream struct {
	jetstream.JetStream

	mu       sync.Mutex
	failures int
	err      error
	attempts int
	optCount []int
}

func (f *flakyJetStream) Publish(_ context.Context, _ string, _ []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	f.optCount = append(f.optCount, len(opts))
	if f.attempts <= f.failures {
		return nil, f.err
	}
	return &jetstream.PubAck{Stream: defaultStream, Sequence: uint64(f.attempts)}, nil
}

type identified struct{ id string }

func (i identified) MessageID() string { return i.id }

type identifiedCodec struct{}

func (identifiedCodec) Encode(any) ([]byte, error) { return []byte("x"), nil }
func (identifiedCodec) Decode([]byte) (any, error) { return nil, nil }

func newRetryBus(js jetstream.JetStream, window time.Duration) *Bus {
	return &Bus{js: js, codec: identifiedCodec{}, retryWindow: window, logf: func(string, ...any) {}}
}

func TestPublishRetriesTransientBrokerOutage(t *testing.T) {
	for _, transient := range []error{
		jetstream.ErrNoStreamResponse,
		natsgo.ErrNoResponders,
		natsgo.ErrTimeout,
		jetstream.ErrJetStreamNotEnabled,
		&jetstream.APIError{Code: 503, Description: "unavailable"},
	} {
		t.Run(transient.Error(), func(t *testing.T) {
			js := &flakyJetStream{failures: 2, err: transient}
			b := newRetryBus(js, 10*time.Second)
			if err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", identified{id: "fact-1"}); err != nil {
				t.Fatalf("Publish: %v", err)
			}
			if js.attempts != 3 {
				t.Fatalf("attempts = %d, want 3", js.attempts)
			}
			for i, n := range js.optCount {
				if n != 1 {
					t.Fatalf("attempt %d carried %d publish options, want the message ID on every attempt", i+1, n)
				}
			}
		})
	}
}

func TestPublishWithoutMessageIDSendsNoOptions(t *testing.T) {
	js := &flakyJetStream{}
	b := newRetryBus(js, time.Second)
	if err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", identified{}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if js.optCount[0] != 0 {
		t.Fatalf("publish options = %d, want 0 for an empty message ID", js.optCount[0])
	}
}

func TestPublishDoesNotRetryPermanentError(t *testing.T) {
	permanent := &jetstream.APIError{Code: 400, Description: "bad request"}
	js := &flakyJetStream{failures: 5, err: permanent}
	b := newRetryBus(js, 10*time.Second)
	err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", identified{id: "fact-1"})
	if !errors.As(err, new(*jetstream.APIError)) {
		t.Fatalf("err = %v, want the API error", err)
	}
	if js.attempts != 1 {
		t.Fatalf("attempts = %d, want 1", js.attempts)
	}
}

func TestPublishGivesUpAfterRetryWindow(t *testing.T) {
	js := &flakyJetStream{failures: 1 << 30, err: jetstream.ErrNoStreamResponse}
	b := newRetryBus(js, 600*time.Millisecond)
	start := time.Now()
	err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", identified{id: "fact-1"})
	if !errors.Is(err, jetstream.ErrNoStreamResponse) {
		t.Fatalf("err = %v, want no stream response", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Publish took %s, want it bounded by the retry window", elapsed)
	}
	if js.attempts < 2 {
		t.Fatalf("attempts = %d, want retries inside the window", js.attempts)
	}
}

func TestPublishStopsRetryingWhenContextEnds(t *testing.T) {
	js := &flakyJetStream{failures: 1 << 30, err: jetstream.ErrNoStreamResponse}
	b := newRetryBus(js, time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := b.Publish(ctx, "app.v1.run.t1.r1.started", identified{id: "fact-1"}); err == nil {
		t.Fatal("Publish succeeded against a dead stream")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Publish took %s after its context ended", elapsed)
	}
}

func TestPublishRetryDisabled(t *testing.T) {
	js := &flakyJetStream{failures: 1, err: jetstream.ErrNoStreamResponse}
	b := newRetryBus(js, 0)
	if err := b.Publish(context.Background(), "app.v1.run.t1.r1.started", identified{id: "fact-1"}); err == nil {
		t.Fatal("Publish succeeded with retries disabled")
	}
	if js.attempts != 1 {
		t.Fatalf("attempts = %d, want 1", js.attempts)
	}
}
