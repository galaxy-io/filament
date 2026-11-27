package heartbeat

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/usage"
)

func TestEmitterPublishesLivenessBeatsWithoutCgroupUsage(t *testing.T) {
	t.Parallel()
	bus := inproc.New()
	sub, err := bus.Subscribe(events.RunPattern("tenant-1", "run-1"), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	em := New(bus, nil, nil, "tenant-1", "run-1", "pipe-1")
	em.readUsage = func() (usage.Sample, bool) { return usage.Sample{}, false }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := em.Start(ctx, 15*time.Millisecond)

	select {
	case msg := <-sub.C():
		fact, decErr := events.Decode(msg)
		_ = msg.Ack()
		if decErr != nil || fact.Name != events.Heartbeat.Name() {
			t.Fatalf("fact = %#v, err = %v; want run.heartbeat", fact, decErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for liveness heartbeat without cgroup usage")
	}
	stop()
}
