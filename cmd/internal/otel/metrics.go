package otel

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"github.com/galaxy-io/filament/telemetry"
)

// backend makes one OTel instrument per declared metric. Instrument creation
// fails only on invalid names; the miss falls back to otel's noop so the
// returned instrument is always callable.
type backend struct{ m metric.Meter }

var _ telemetry.Backend = backend{}

func (b backend) Counter(d telemetry.Desc) telemetry.Counter {
	c, err := b.m.Float64Counter(d.Name, metric.WithDescription(d.Help), metric.WithUnit(d.Unit))
	if err != nil {
		c, _ = noopmetric.Meter{}.Float64Counter(d.Name)
	}
	return counter{c: c, keys: d.Attributes}
}

func (b backend) Histogram(d telemetry.Desc) telemetry.Histogram {
	opts := []metric.Float64HistogramOption{metric.WithDescription(d.Help), metric.WithUnit(d.Unit)}
	if len(d.Buckets) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(d.Buckets...))
	}
	h, err := b.m.Float64Histogram(d.Name, opts...)
	if err != nil {
		h, _ = noopmetric.Meter{}.Float64Histogram(d.Name)
	}
	return histogram{h: h, keys: d.Attributes}
}

func (b backend) Gauge(d telemetry.Desc) telemetry.Gauge {
	g, err := b.m.Float64Gauge(d.Name, metric.WithDescription(d.Help), metric.WithUnit(d.Unit))
	if err != nil {
		g, _ = noopmetric.Meter{}.Float64Gauge(d.Name)
	}
	return gauge{g: g, keys: d.Attributes}
}

type counter struct {
	c    metric.Float64Counter
	keys []string
}

func (c counter) Add(ctx context.Context, v float64, values ...string) {
	c.c.Add(ctx, v, attrs(c.keys, values))
}

type histogram struct {
	h    metric.Float64Histogram
	keys []string
}

func (h histogram) Observe(ctx context.Context, v float64, values ...string) {
	h.h.Record(ctx, v, attrs(h.keys, values))
}

type gauge struct {
	g    metric.Float64Gauge
	keys []string
}

func (g gauge) Set(ctx context.Context, v float64, values ...string) {
	g.g.Record(ctx, v, attrs(g.keys, values))
}

// attrs pairs a metric's attribute keys with one measurement's values. A
// value with no key, a key with no value, or an empty value is left out.
func attrs(keys, values []string) metric.MeasurementOption {
	n := min(len(keys), len(values))
	kvs := make([]attribute.KeyValue, 0, n)
	for i := range n {
		if values[i] != "" {
			kvs = append(kvs, attribute.String(keys[i], values[i]))
		}
	}
	if len(kvs) == 0 {
		return unattributed
	}
	return metric.WithAttributeSet(attribute.NewSet(kvs...))
}

// unattributed is the option of a measurement with no attributes.
var unattributed = metric.WithAttributeSet(*attribute.EmptySet())
