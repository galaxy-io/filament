// Package telemetry declares every metric Filament records and the events
// that record them. A Backend turns them into a metrics system's instruments.
package telemetry

import "context"

// Kind is the kind of instrument a metric is recorded with.
type Kind int

const (
	// KindCounter only goes up.
	KindCounter Kind = iota
	// KindHistogram records a distribution of values.
	KindHistogram
	// KindGauge holds the last value set.
	KindGauge
)

func (k Kind) String() string {
	switch k {
	case KindCounter:
		return "counter"
	case KindHistogram:
		return "histogram"
	default:
		return "gauge"
	}
}

// Desc declares one metric: its name, its UCUM unit, what it measures, its
// attribute keys in the order values are given when it is recorded, and for a
// histogram the upper bounds of its buckets. An empty attribute value means
// the attribute does not apply to that measurement and is left off.
type Desc struct {
	Name       string
	Unit       string
	Help       string
	Kind       Kind
	Attributes []string
	Buckets    []float64
}

// Backend makes the instrument each metric is recorded with. It is asked once
// per metric, when a Set is built.
type Backend interface {
	Counter(Desc) Counter
	Histogram(Desc) Histogram
	Gauge(Desc) Gauge
}

// Counter adds to a metric. values are its attribute values, in
// Desc.Attributes order. ctx carries the active span, if any, for exemplars.
type Counter interface {
	Add(ctx context.Context, v float64, values ...string)
}

// Histogram records one observation of a metric.
type Histogram interface {
	Observe(ctx context.Context, v float64, values ...string)
}

// Gauge sets a metric's current value.
type Gauge interface {
	Set(ctx context.Context, v float64, values ...string)
}

// catalog is every metric declared in this package, in declaration order.
var catalog []Desc

// Catalog returns every metric Filament records.
func Catalog() []Desc {
	return append([]Desc(nil), catalog...)
}

func declare(kind Kind, name, unit, help string, buckets []float64, attributes []string) Desc {
	d := Desc{Name: name, Unit: unit, Help: help, Kind: kind, Attributes: attributes, Buckets: buckets}
	catalog = append(catalog, d)
	return d
}

func counter(name, unit, help string, attributes ...string) Desc {
	return declare(KindCounter, name, unit, help, nil, attributes)
}

func histogram(name, unit, help string, buckets []float64, attributes ...string) Desc {
	return declare(KindHistogram, name, unit, help, buckets, attributes)
}

func gauge(name, unit, help string, attributes ...string) Desc {
	return declare(KindGauge, name, unit, help, nil, attributes)
}
