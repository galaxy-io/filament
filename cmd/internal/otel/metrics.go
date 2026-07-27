package otel

import (
	"context"
	"math"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"github.com/galaxy-io/filament"
)

type metrics struct{ m metric.Meter }

var _ filament.Metrics = metrics{}

// Instrument creation fails only on invalid names; the miss falls back to
// otel's noop so the returned instrument is always callable.

func (x metrics) Counter(name string, labels ...filament.Label) filament.Counter {
	c, err := x.m.Float64Counter(name)
	if err != nil {
		c, _ = noopmetric.Meter{}.Float64Counter(name)
	}
	return counter{c: c, opt: attrs(labels)}
}

func (x metrics) Gauge(name string, labels ...filament.Label) filament.Gauge {
	g, err := x.m.Float64Gauge(name)
	if err != nil {
		g, _ = noopmetric.Meter{}.Float64Gauge(name)
	}
	return &gauge{g: g, opt: attrs(labels)}
}

func (x metrics) Histogram(name string, labels ...filament.Label) filament.Histogram {
	h, err := x.m.Float64Histogram(name)
	if err != nil {
		h, _ = noopmetric.Meter{}.Float64Histogram(name)
	}
	return histogram{h: h, opt: attrs(labels)}
}

func attrs(labels []filament.Label) metric.MeasurementOption {
	kvs := make([]attribute.KeyValue, len(labels))
	for i, l := range labels {
		kvs[i] = attribute.String(l.Key, l.Value)
	}
	return metric.WithAttributeSet(attribute.NewSet(kvs...))
}

type counter struct {
	c   metric.Float64Counter
	opt metric.MeasurementOption
}

func (c counter) Inc()          { c.c.Add(context.Background(), 1, c.opt) }
func (c counter) Add(v float64) { c.c.Add(context.Background(), v, c.opt) }

// gauge tracks its own value so Inc/Dec can report an absolute measurement.
type gauge struct {
	g   metric.Float64Gauge
	opt metric.MeasurementOption
	val atomic.Uint64 // float64 bits
}

func (g *gauge) Set(v float64) {
	g.val.Store(math.Float64bits(v))
	g.g.Record(context.Background(), v, g.opt)
}

func (g *gauge) Inc() { g.add(1) }
func (g *gauge) Dec() { g.add(-1) }

func (g *gauge) add(d float64) {
	for {
		old := g.val.Load()
		v := math.Float64frombits(old) + d
		if g.val.CompareAndSwap(old, math.Float64bits(v)) {
			g.g.Record(context.Background(), v, g.opt)
			return
		}
	}
}

type histogram struct {
	h   metric.Float64Histogram
	opt metric.MeasurementOption
}

func (h histogram) Observe(v float64) { h.h.Record(context.Background(), v, h.opt) }
