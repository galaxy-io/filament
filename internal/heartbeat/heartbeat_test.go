package heartbeat

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/usage"
)

type recordingMetrics struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

func (m *recordingMetrics) Counter(name string, _ ...filament.Label) filament.Counter {
	return metricCounter{m: m, name: name}
}

func (m *recordingMetrics) Gauge(name string, _ ...filament.Label) filament.Gauge {
	return metricGauge{m: m, name: name}
}

func (m *recordingMetrics) Histogram(string, ...filament.Label) filament.Histogram {
	return metricHistogram{}
}

type metricCounter struct {
	m    *recordingMetrics
	name string
}

func (c metricCounter) Inc()          { c.Add(1) }
func (c metricCounter) Add(v float64) { c.m.mu.Lock(); c.m.counters[c.name] += v; c.m.mu.Unlock() }

type metricGauge struct {
	m    *recordingMetrics
	name string
}

func (g metricGauge) Set(v float64) { g.m.mu.Lock(); g.m.gauges[g.name] = v; g.m.mu.Unlock() }
func (g metricGauge) Inc()          { g.m.mu.Lock(); g.m.gauges[g.name]++; g.m.mu.Unlock() }
func (g metricGauge) Dec()          { g.m.mu.Lock(); g.m.gauges[g.name]--; g.m.mu.Unlock() }

type metricHistogram struct{}

func (metricHistogram) Observe(float64) {}

func TestEmitterPublishesLivenessBeatsWithoutCgroupUsage(t *testing.T) {
	bus := inproc.New()
	sub, err := bus.Subscribe(events.RunPattern("tenant-1", "run-1"), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	em := newEmitter(bus, nil, nil, "tenant-1", "run-1", "pipe-1", func() (usage.Sample, bool) {
		return usage.Sample{}, false
	})

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

func TestEmitterSeedsInitialCPUBaselineOnConstruction(t *testing.T) {
	bus := inproc.New()
	mx := &recordingMetrics{
		counters: map[string]float64{},
		gauges:   map[string]float64{},
	}
	samples := []usage.Sample{
		{CPUSeconds: 42.0, MemoryBytes: 1024},
		{CPUSeconds: 43.5, MemoryBytes: 2048},
		{CPUSeconds: 45.0, MemoryBytes: 4096},
	}
	idx := 0
	em := newEmitter(bus, mx, nil, "tenant-1", "run-1", "pipe-1", func() (usage.Sample, bool) {
		s := samples[idx]
		if idx+1 < len(samples) {
			idx++
		}
		return s, true
	})

	em.beat(context.Background(), true)
	em.beat(context.Background(), false)

	got := mx.counters["filament_worker_cpu_seconds_total"]
	if math.Abs(got-3.0) > 1e-9 {
		t.Fatalf("cpu counter delta = %v, want 3.0 (seeded from initial 42.0 baseline)", got)
	}
}
