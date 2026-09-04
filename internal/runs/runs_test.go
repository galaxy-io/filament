package runs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
)

type failPublishBus struct {
	eventbus.Bus
	failures int
}

func (b *failPublishBus) Publish(ctx context.Context, subject string, payload any) error {
	if b.failures > 0 {
		b.failures--
		return errors.New("publish unavailable")
	}
	return b.Bus.Publish(ctx, subject, payload)
}

type failDeleteStore struct {
	filament.DataStore
	failures int
}

func (s *failDeleteStore) DeleteRun(ctx context.Context, tenant filament.TenantID, id filament.RunID) error {
	if s.failures > 0 {
		s.failures--
		return errors.New("delete unavailable")
	}
	return s.DataStore.DeleteRun(ctx, tenant, id)
}

func TestIDForReturnsUUID(t *testing.T) {
	req := filament.RunRequest{Tenant: filament.DefaultTenantID, IdempotencyKey: "client-token"}

	first := IDFor(req)
	second := IDFor(req)
	if first != second {
		t.Fatalf("idempotent IDs differ: %q != %q", first, second)
	}
	if _, err := uuid.Parse(string(first)); err != nil {
		t.Fatalf("IDFor returned a non-UUID %q: %v", first, err)
	}
	if random := IDFor(filament.RunRequest{}); random == first {
		t.Fatalf("random ID unexpectedly matched deterministic ID %q", first)
	} else if _, err := uuid.Parse(string(random)); err != nil {
		t.Fatalf("IDFor returned a non-UUID random ID %q: %v", random, err)
	}
}

func TestSubmitRetryRedispatchesStrandedRequestedRun(t *testing.T) {
	ctx := context.Background()
	baseBus := inproc.New()
	sub, err := baseBus.Subscribe(events.SubjectPattern(events.RunRequested), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	bus := &failPublishBus{Bus: baseBus, failures: 1}
	store := &failDeleteStore{DataStore: memory.New(), failures: 1}
	req := filament.RunRequest{Tenant: filament.DefaultTenantID, IdempotencyKey: "retry-me"}
	id := IDFor(req)

	if _, err := Submit(ctx, bus, store, filament.RunSubmission{Request: req}); err == nil || !strings.Contains(err.Error(), "delete undispatched") {
		t.Fatalf("first Submit error = %v, want joined dispatch/delete failure", err)
	}
	state, err := store.LoadRun(ctx, req.Tenant, id)
	if err != nil || state.Status != filament.RunRequested {
		t.Fatalf("stranded state = %#v, err = %v", state, err)
	}

	got, err := Submit(ctx, bus, store, filament.RunSubmission{Request: req})
	if err != nil || got != id {
		t.Fatalf("retry Submit = %q, %v; want %q, nil", got, err, id)
	}
	select {
	case msg := <-sub.C():
		fact, decodeErr := events.Decode(msg)
		_ = msg.Ack()
		if decodeErr != nil || fact.Run != id {
			t.Fatalf("redispatched fact = %#v, err = %v", fact, decodeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("retry did not publish run.requested")
	}
}
