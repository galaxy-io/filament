package heartbeat

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/usage"
)

type testCounter struct {
	mu  sync.Mutex
	sum float64
}

func (c *testCounter) Inc()              { c.Add(1) }
func (c *testCounter) Add(delta float64) { c.mu.Lock(); c.sum += delta; c.mu.Unlock() }

type testGauge struct{ filament.Gauge }

func (testGauge) Set(float64) {}

type testHist struct{ filament.Histogram }

func (testHist) Observe(float64) {}

type testMetrics struct {
	cpu testCounter
}

func (m *testMetrics) Counter(name string, _ ...filament.Label) filament.Counter {
	if name == "filament_worker_cpu_seconds_total" {
		return &m.cpu
	}
	return &testCounter{}
}
func (m *testMetrics) Gauge(string, ...filament.Label) filament.Gauge         { return testGauge{} }
func (m *testMetrics) Histogram(string, ...filament.Label) filament.Histogram { return testHist{} }

func TestEmitterPublishesLivenessBeatsWithoutCgroupUsage(t *testing.T) {
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

func TestEmitterDoesNotOvercountProcessCPUAcrossRuns(t *testing.T) {
	resetProcessCPUForTest()
	bus := inproc.New()
	mx := &testMetrics{}
	ctx := context.Background()

	// Run 1 starts when cumulative cgroup CPU is already 10s, ends at 14s (delta = 4s).
	curCPU := 10.0
	em1 := New(bus, mx, nil, "tenant-1", "run-1", "pipe-1")
	em1.readUsage = func() (usage.Sample, bool) {
		return usage.Sample{CPUSeconds: curCPU, MemoryBytes: 1024}, true
	}
	stop1 := em1.Start(ctx, time.Hour)
	curCPU = 14.0
	stop1()

	// Run 2 starts in the same worker process at 14s, ends at 17s (delta = 3s, total = 7s, not 14+17).
	em2 := New(bus, mx, nil, "tenant-1", "run-2", "pipe-1")
	em2.readUsage = func() (usage.Sample, bool) {
		return usage.Sample{CPUSeconds: curCPU, MemoryBytes: 2048}, true
	}
	stop2 := em2.Start(ctx, time.Hour)
	curCPU = 17.0
	stop2()

	if mx.cpu.sum != 7.0 {
		t.Fatalf("filament_worker_cpu_seconds_total delta = %v, want 7.0", mx.cpu.sum)
	}
}
