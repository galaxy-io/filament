package tracker

import (
	"context"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/telemetry"
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
func (m *Module) observeRun(ctx context.Context, env events.Envelope, status, errorType string, l runLabels) {
	run := telemetry.Run{Pipeline: l.pipeline, Source: l.source, Sink: l.sink, Status: status, ErrorType: errorType, EndedAt: env.At}
	if !l.startedAt.IsZero() {
		run.Took = env.At.Sub(l.startedAt)
	}
	m.mx.RunEnded(ctx, run)
}

// observeRecords counts the records a bounded run read or rejected, and
// reports whether f was a fact of either. They fold into no run state, so a
// run that cannot be read is skipped rather than holding up the facts behind
// it. A continuous run's worker counts its own.
func (m *Module) observeRecords(ctx context.Context, f events.Fact) bool {
	var read, rejected int64
	var errorType string
	switch d := f.Data.(type) {
	case events.BatchBufferedEvent:
		read = d.Records
	case events.BatchRejectedEvent:
		rejected, errorType = d.Records, d.ErrorType
	default:
		return false
	}
	if m.mx == nil {
		return true
	}
	info, known, err := m.runInfo(ctx, f.Tenant, f.Run)
	if err != nil || !known || info.mode == filament.ExecutionContinuous {
		return true
	}
	m.mx.RecordsRead(ctx, info.pipeline, f.Resource, read)
	m.mx.RecordsRejected(ctx, info.pipeline, f.Resource, errorType, rejected)
	return true
}

// observeQueue records how long a run waited from submission to its start.
func (m *Module) observeQueue(ctx context.Context, pipeline string, queued time.Duration) {
	m.mx.RunStarted(ctx, pipeline, queued)
}

// observeBatch accumulates written records and bytes as batch facts land.
func (m *Module) observeBatch(ctx context.Context, env events.Envelope, pipeline string, records, bytes int64) {
	m.mx.BatchWritten(ctx, pipeline, env.Resource, records, bytes)
}

// observeCheckpointFailure counts a cursor persist that failed and was only logged.
func (m *Module) observeCheckpointFailure(ctx context.Context) {
	m.mx.CheckpointFlushFailed(ctx)
}
