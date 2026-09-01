package main

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

// heartbeat samples the pod's cgroup usage on an interval, publishing each
// sample as a run.heartbeat fact and mirroring it onto the OTel instruments.
// Instruments are labeled by tenant and pipeline, never run — per-run detail
// belongs to the facts; per-run label cardinality would drown the backend.
type heartbeat struct {
	bus      eventbus.Bus
	mx       filament.Metrics
	log      filament.Logger
	tenant   filament.TenantID
	run      filament.RunID
	pipeline string

	lastCPU float64
}

// start launches the sampling loop. The returned stop function ends the loop
// and takes one final sample, so short runs still report usage at least once.
func (h *heartbeat) start(ctx context.Context, every time.Duration) (stop func()) {
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
				h.beat(ctx)
			}
		}
	}()
	return func() {
		close(done)
		<-finished
		h.beat(ctx)
	}
}

func (h *heartbeat) beat(ctx context.Context) {
	s, ok := usage.Read()
	if !ok {
		return // no cgroup filesystem — a dev run outside a pod
	}
	err := events.Emit(ctx, h.bus, events.Heartbeat,
		events.Envelope{Tenant: h.tenant, Run: h.run, At: time.Now()},
		events.HeartbeatEvent{
			CPUSeconds:      s.CPUSeconds,
			MemoryBytes:     s.MemoryBytes,
			MemoryPeakBytes: s.MemoryPeakBytes,
		})
	if err != nil && h.log != nil {
		h.log.Warn("heartbeat not published",
			filament.Field{Key: "event.name", Value: "worker.heartbeat.publish_failed"},
			filament.Field{Key: "error", Value: err.Error()})
	} else if h.log != nil {
		h.log.Trace("heartbeat published",
			filament.Field{Key: "event.name", Value: "worker.heartbeat.published"},
			filament.Field{Key: "cpu_seconds", Value: s.CPUSeconds},
			filament.Field{Key: "memory_bytes", Value: s.MemoryBytes},
			filament.Field{Key: "memory_peak_bytes", Value: s.MemoryPeakBytes})
	}
	if h.mx == nil {
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

// heartbeatInterval is HEARTBEAT_SECONDS or the default.
func heartbeatInterval() time.Duration {
	if v := os.Getenv("HEARTBEAT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultHeartbeatSeconds * time.Second
}
