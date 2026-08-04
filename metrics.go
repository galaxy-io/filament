package filament

import "time"

// Metric is an aggregatable measure over runs. RunCount/RunRecords/RunBytes
// are sums; RunDuration is the mean duration over terminal runs. Mirrors
// metrics.v1.Metric (api/metrics/v1/metrics.proto).
type Metric int

// Metric values, mirroring metrics.v1.Metric's real (non-UNSPECIFIED) entries.
const (
	MetricRunCount Metric = iota
	MetricRunRecords
	MetricRunBytes
	MetricRunDuration
)

// MetricsDimension names a runs column usable for grouping and filtering.
// Mirrors metrics.v1.Dimension; the zero value means unset ("no grouping",
// one total series/row with key "").
type MetricsDimension int

// MetricsDimension values, mirroring metrics.v1.Dimension.
const (
	DimensionUnspecified MetricsDimension = iota
	DimensionTenantID
	DimensionPipelineID
	DimensionStatus
)

// MetricsGranularity is the bucket width for a timeseries query. Mirrors
// metrics.v1.MetricsGranularity.
type MetricsGranularity int

// MetricsGranularity values, mirroring metrics.v1.MetricsGranularity's real
// (non-UNSPECIFIED) entries.
const (
	GranularityHour MetricsGranularity = iota
	GranularityDay
)

// MetricsFilter narrows a query to rows whose dimension matches one of values.
type MetricsFilter struct {
	Dimension MetricsDimension
	Values    []string
}

// RunTimeseriesQuery parameterizes a bucketed metrics query over runs.
type RunTimeseriesQuery struct {
	Tenant          TenantID
	Metrics         []Metric
	Since, Until    time.Time
	Granularity     MetricsGranularity
	TZOffsetMinutes int
	// GroupBy splits the result into one series per dimension value;
	// DimensionUnspecified means one total series with key "".
	GroupBy MetricsDimension
	Filters []MetricsFilter
}

// RunAggregateQuery parameterizes a single-window metrics query over runs.
type RunAggregateQuery struct {
	Tenant       TenantID
	Metrics      []Metric
	Since, Until time.Time
	// GroupBy splits the result into one row per dimension value;
	// DimensionUnspecified means one total row with key "".
	GroupBy MetricsDimension
	Filters []MetricsFilter
}

// TimeseriesPoint is one bucket's metric values, positionally aligned with
// the query's Metrics.
type TimeseriesPoint struct {
	BucketStart time.Time
	Values      []float64
}

// RunTimeseries is one grouped series: Key is the GroupBy dimension's value
// ("" when the query has no GroupBy), Points are dense — every bucket in
// [Since, Until) is present, zero-filled, ascending.
type RunTimeseries struct {
	Key    string
	Points []TimeseriesPoint
}

// RunAggregateRow is one grouped row's metric values, positionally aligned
// with the query's Metrics. Key is "" when the query has no GroupBy.
type RunAggregateRow struct {
	Key    string
	Values []float64
}
