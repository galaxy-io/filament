package tracker

import (
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// runLabels are the low-cardinality identifiers captured from run state while
// it is loaded, so metrics can be recorded after the fold without re-reading.
type runLabels struct {
	pipeline  string
	source    string
	sink      string
	startedAt time.Time
}

func labelsFor(r *filament.RunState) runLabels {
	return runLabels{
		pipeline:  r.Request.PipelineID,
		source:    r.Request.Source.Connector,
		sink:      r.Request.Sink.Connector,
		startedAt: r.StartedAt,
	}
}

// observeRun records a run's terminal outcome: the runs counter, the duration
// histogram, and on completion the freshness gauge alerting keys on.
func (m *Module) observeRun(env events.Envelope, status string, l runLabels) {
	if m.mx == nil {
		return
	}
	m.mx.Counter("filament_runs_total",
		filament.Label{Key: "status", Value: status},
		filament.Label{Key: "source", Value: l.source},
		filament.Label{Key: "sink", Value: l.sink},
		filament.Label{Key: "pipeline", Value: l.pipeline},
	).Inc()
	if !l.startedAt.IsZero() && env.At.After(l.startedAt) {
		m.mx.Histogram("filament_run_duration_seconds",
			filament.Label{Key: "status", Value: status},
			filament.Label{Key: "pipeline", Value: l.pipeline},
		).Observe(env.At.Sub(l.startedAt).Seconds())
	}
	if status == "completed" {
		m.mx.Gauge("filament_last_success_timestamp_seconds",
			filament.Label{Key: "pipeline", Value: l.pipeline},
		).Set(float64(env.At.Unix()))
	}
}

// observeBatch accumulates written records and bytes as batch facts land.
func (m *Module) observeBatch(env events.Envelope, pipeline string, records, bytes int64) {
	if m.mx == nil {
		return
	}
	labels := []filament.Label{
		{Key: "pipeline", Value: pipeline},
		{Key: "resource", Value: env.Resource},
	}
	m.mx.Counter("filament_records_written_total", labels...).Add(float64(records))
	m.mx.Counter("filament_bytes_written_total", labels...).Add(float64(bytes))
}

// observeCheckpointFailure counts a cursor persist that failed and was only logged.
func (m *Module) observeCheckpointFailure() {
	if m.mx == nil {
		return
	}
	m.mx.Counter("filament_checkpoint_flush_failures_total").Inc()
}
