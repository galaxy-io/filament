package zerobus

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// telemetry holds optional OpenTelemetry instruments. They export through the
// process-wide meter provider the engine configures from OTEL_* env vars, and
// are no-ops when OpenTelemetry is disabled.
type telemetry struct {
	flushLatency metric.Float64Histogram
	batches      metric.Int64Counter
	errors       metric.Int64Counter
}

// newTelemetry builds the meter instruments from the process-wide provider.
// Instrument construction errors are ignored: a nil instrument records nothing.
func newTelemetry() *telemetry {
	m := otel.Meter("github.com/galaxy-io/filament/connectors/databricks/zerobus")
	flush, _ := m.Float64Histogram("zerobus.flush.latency",
		metric.WithUnit("ms"), metric.WithDescription("Zerobus stream flush latency in milliseconds"))
	batches, _ := m.Int64Counter("zerobus.batches.ingested",
		metric.WithDescription("Arrow batches ingested into Zerobus"))
	errs, _ := m.Int64Counter("zerobus.errors",
		metric.WithDescription("Zerobus ingest and flush errors"))
	return &telemetry{flushLatency: flush, batches: batches, errors: errs}
}

// recordFlush records a stream flush latency for the resource.
func (t *telemetry) recordFlush(ctx context.Context, resource string, d time.Duration) {
	if t == nil || t.flushLatency == nil {
		return
	}
	t.flushLatency.Record(ctx, float64(d.Milliseconds()), metric.WithAttributes(attribute.String("resource", resource)))
}

// recordBatch counts one ingested Arrow batch for the resource.
func (t *telemetry) recordBatch(ctx context.Context, resource string) {
	if t == nil || t.batches == nil {
		return
	}
	t.batches.Add(ctx, 1, metric.WithAttributes(attribute.String("resource", resource)))
}

// recordError counts one ingest or flush error, labeled by whether it is retryable.
func (t *telemetry) recordError(ctx context.Context, resource string, isRetryable bool) {
	if t == nil || t.errors == nil {
		return
	}
	t.errors.Add(ctx, 1, metric.WithAttributes(
		attribute.String("resource", resource), attribute.Bool("retryable", isRetryable)))
}
