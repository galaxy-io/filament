package inproc

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament/eventbus"
)

// Publish overhead with no subscribers: seq + log + split + match loop.
// Bounded log keeps memory flat across b.N.
func BenchmarkPublishNoSubscribers(b *testing.B) {
	bus := New(WithMaxLog(4096))
	defer func() { _ = bus.Close() }()
	const subj = "app.v1.batch.t1.r1.written"
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		_ = bus.Publish(ctx, subj, 1)
	}
}

// Fan-out to 8 draining subscribers — the realistic per-fact cost.
func BenchmarkPublishFanout8(b *testing.B) {
	bus := New(WithMaxLog(4096), WithBuffer(1<<14))
	defer func() { _ = bus.Close() }()
	for range 8 {
		s, err := bus.Subscribe("app.v1.>", eventbus.SubOpts{})
		if err != nil {
			b.Fatal(err)
		}
		go func(s eventbus.Subscription) {
			for range s.C() { //nolint:revive // draining
			}
		}(s)
	}
	const subj = "app.v1.batch.t1.r1.written"
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		_ = bus.Publish(ctx, subj, 1)
	}
}

// Concurrent publishers contending on the bus's single mutex, with 4 drained
// subscribers so the match loop runs under the lock.
func BenchmarkPublishParallel(b *testing.B) {
	bus := New(WithMaxLog(8192), WithBuffer(1<<16))
	defer func() { _ = bus.Close() }()
	for range 4 {
		s, err := bus.Subscribe("app.v1.>", eventbus.SubOpts{})
		if err != nil {
			b.Fatal(err)
		}
		go func(s eventbus.Subscription) {
			for range s.C() { //nolint:revive // draining
			}
		}(s)
	}
	const subj = "app.v1.batch.t1.r1.written"
	ctx := context.Background()

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bus.Publish(ctx, subj, 1)
		}
	})
}
