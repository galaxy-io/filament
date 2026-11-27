// Package heartbeat publishes periodic run.heartbeat facts while a bounded run
// executes, both to report cgroup resource usage when available and to advance
// RunState.UpdatedAt so the control-plane reaper can distinguish live runs
// from dead workers under any dispatch mode.
package heartbeat

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/usage"
)

const defaultHeartbeatSeconds = 30

// Emitter publishes periodic run.heartbeat facts for one active run.
type Emitter struct {
	bus      eventbus.Bus
	mx       filament.Metrics
	log      filament.Logger
	tenant   filament.TenantID
	run      filament.RunID
	pipeline string

	readUsage func() (usage.Sample, bool)
	lastCPU   float64
}

// New constructs a heartbeat emitter for the given run.
func New(
	bus eventbus.Bus,
	mx filament.Metrics,
	log filament.Logger,
	tenant filament.TenantID,
	run filament.RunID,
	pipeline string,
) *Emitter {
	return &Emitter{
		bus:       bus,
		mx:        mx,
		log:       log,
		tenant:    tenant,
		run:       run,
		pipeline:  pipeline,
		readUsage: usage.Read,
	}
}

// Interval returns HEARTBEAT_SECONDS or the 30s default.
func Interval() time.Duration {
	if v := os.Getenv("HEARTBEAT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultHeartbeatSeconds * time.Second
}

// Start launches the periodic heartbeat loop. Each interval tick emits a
// run.heartbeat fact even when cgroup metrics are unavailable so long-lived
// worker processes refresh RunState.UpdatedAt. On stop, one final usage sample
// is emitted only when cgroup usage is readable and the context is still live.
func (h *Emitter) Start(ctx context.Context, every time.Duration) (stop func()) {
	if h == nil || h.bus == nil || every <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				h.beat(ctx, true)
			}
		}
	}()
	return func() {
		close(done)
		<-finished
		if ctx.Err() == nil {
			h.beat(ctx, false)
		}
	}
}

func (h *Emitter) beat(ctx context.Context, livenessTick bool) {
	read := h.readUsage
	if read == nil {
		read = usage.Read
	}
	s, ok := read()
	if !ok && !livenessTick {
		return
	}
	var ev events.HeartbeatEvent
	if ok {
		ev = events.HeartbeatEvent{
			CPUSeconds:      s.CPUSeconds,
			MemoryBytes:     s.MemoryBytes,
			MemoryPeakBytes: s.MemoryPeakBytes,
		}
	}
	err := events.Emit(ctx, h.bus, events.Heartbeat,
		events.Envelope{Tenant: h.tenant, Run: h.run, At: time.Now()},
		ev)
	if err != nil && h.log != nil {
		h.log.Warn("heartbeat not published",
			filament.Field{Key: "event.name", Value: "worker.heartbeat.publish_failed"},
			filament.Field{Key: "error", Value: err.Error()})
	} else if h.log != nil {
		h.log.Trace("heartbeat published",
			filament.Field{Key: "event.name", Value: "worker.heartbeat.published"},
			filament.Field{Key: "cpu_seconds", Value: ev.CPUSeconds},
			filament.Field{Key: "memory_bytes", Value: ev.MemoryBytes},
			filament.Field{Key: "memory_peak_bytes", Value: ev.MemoryPeakBytes})
	}
	if !ok || h.mx == nil {
		return
	}
	labels := []filament.Label{
		{Key: "tenant", Value: string(h.tenant)},
		{Key: "pipeline", Value: h.pipeline},
	}
	if delta := s.CPUSeconds - h.lastCPU; delta > 0 {
		h.mx.Counter("filament_worker_cpu_seconds_total", labels...).Add(delta)
	}
	h.lastCPU = s.CPUSeconds
	h.mx.Gauge("filament_worker_memory_bytes", labels...).Set(float64(s.MemoryBytes))
}
