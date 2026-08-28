package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	metricsv1 "github.com/galaxy-io/filament/api/metrics/v1"
	"github.com/galaxy-io/filament/api/metrics/v1/metricsv1connect"
)

var _ metricsv1connect.MetricsServiceHandler = (*Server)(nil)

// QueryAggregate returns one row per group_by value (a single "" key when
// group_by is unset) over [since_ms, until_ms).
func (a *Server) QueryAggregate(ctx context.Context, req *connect.Request[metricsv1.QueryAggregateRequest]) (*connect.Response[metricsv1.QueryAggregateResponse], error) {
	if a.metrics == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no metrics store configured"))
	}
	m := req.Msg
	metrics, err := metricsFromProto(m.GetMetrics())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	filters, err := metricFiltersFromProto(m.GetFilters())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	groupBy := metricDimensionFromProto(m.GetGroupBy())

	rows, err := a.metrics.QueryRunAggregate(ctx, filament.RunAggregateQuery{
		Tenant:  filament.TenantID(m.GetTenantId()),
		Metrics: metrics,
		Since:   time.UnixMilli(m.GetSinceMs()),
		Until:   untilOrNow(m.GetUntilMs()),
		GroupBy: groupBy,
		Filters: filters,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*metricsv1.AggregateRow, len(rows))
	for i, r := range rows {
		key := r.Key
		if groupBy == filament.DimensionStatus {
			key = keyToProtoStatus(key)
		}
		out[i] = &metricsv1.AggregateRow{Key: key, Values: r.Values}
	}
	return connect.NewResponse(&metricsv1.QueryAggregateResponse{Rows: out}), nil
}

// QueryTimeseries returns one dense, zero-filled, ascending series per
// group_by value (a single "" series when group_by is unset) bucketed at the
// requested granularity over [since_ms, until_ms).
func (a *Server) QueryTimeseries(ctx context.Context, req *connect.Request[metricsv1.QueryTimeseriesRequest]) (*connect.Response[metricsv1.QueryTimeseriesResponse], error) {
	if a.metrics == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no metrics store configured"))
	}
	m := req.Msg
	metrics, err := metricsFromProto(m.GetMetrics())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	granularity, err := metricGranularityFromProto(m.GetGranularity())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	filters, err := metricFiltersFromProto(m.GetFilters())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	groupBy := metricDimensionFromProto(m.GetGroupBy())

	series, err := a.metrics.QueryRunTimeseries(ctx, filament.RunTimeseriesQuery{
		Tenant:          filament.TenantID(m.GetTenantId()),
		Metrics:         metrics,
		Since:           time.UnixMilli(m.GetSinceMs()),
		Until:           untilOrNow(m.GetUntilMs()),
		Granularity:     granularity,
		TZOffsetMinutes: int(m.GetTzOffsetMinutes()),
		GroupBy:         groupBy,
		Filters:         filters,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*metricsv1.Timeseries, len(series))
	for i, ts := range series {
		points := make([]*metricsv1.TimeseriesPoint, len(ts.Points))
		for j, p := range ts.Points {
			points[j] = &metricsv1.TimeseriesPoint{BucketStartMs: p.BucketStart.UnixMilli(), Values: p.Values}
		}
		key := ts.Key
		if groupBy == filament.DimensionStatus {
			key = keyToProtoStatus(key)
		}
		out[i] = &metricsv1.Timeseries{Key: key, Points: points}
	}
	return connect.NewResponse(&metricsv1.QueryTimeseriesResponse{Series: out}), nil
}

func metricFromProto(m metricsv1.Metric) (filament.Metric, error) {
	switch m {
	case metricsv1.Metric_METRIC_RUN_COUNT:
		return filament.MetricRunCount, nil
	case metricsv1.Metric_METRIC_RUN_RECORDS:
		return filament.MetricRunRecords, nil
	case metricsv1.Metric_METRIC_RUN_BYTES:
		return filament.MetricRunBytes, nil
	case metricsv1.Metric_METRIC_RUN_DURATION:
		return filament.MetricRunDuration, nil
	case metricsv1.Metric_METRIC_RUN_MEMORY_USAGE:
		return filament.MetricRunMemoryUsage, nil
	case metricsv1.Metric_METRIC_RUN_CPU_USAGE:
		return filament.MetricRunCPUUsage, nil
	default:
		return 0, fmt.Errorf("metrics: unsupported metric %v", m)
	}
}

func metricsFromProto(ms []metricsv1.Metric) ([]filament.Metric, error) {
	if len(ms) == 0 {
		return nil, fmt.Errorf("metrics: at least one metric is required")
	}
	out := make([]filament.Metric, len(ms))
	for i, m := range ms {
		fm, err := metricFromProto(m)
		if err != nil {
			return nil, err
		}
		out[i] = fm
	}
	return out, nil
}

func metricDimensionFromProto(d metricsv1.MetricDimension) filament.MetricsDimension {
	switch d {
	case metricsv1.MetricDimension_METRIC_DIMENSION_TENANT_ID:
		return filament.DimensionTenantID
	case metricsv1.MetricDimension_METRIC_DIMENSION_PIPELINE_ID:
		return filament.DimensionPipelineID
	case metricsv1.MetricDimension_METRIC_DIMENSION_STATUS:
		return filament.DimensionStatus
	default:
		return filament.DimensionUnspecified
	}
}

// runStatusFromProto translates one DIMENSION_STATUS value from the wire
// encoding (ingestion.v1.RunStatus) to the domain ordinal the runs.status
// column stores. The strict inverse of convert.go's runStatusToProto: an
// unknown value errors rather than being silently dropped.
func runStatusFromProto(v ingestionv1.RunStatus) (filament.RunStatus, error) {
	switch v {
	case ingestionv1.RunStatus_RUN_STATUS_REQUESTED:
		return filament.RunRequested, nil
	case ingestionv1.RunStatus_RUN_STATUS_RUNNING:
		return filament.RunRunning, nil
	case ingestionv1.RunStatus_RUN_STATUS_COMPLETED:
		return filament.RunCompleted, nil
	case ingestionv1.RunStatus_RUN_STATUS_FAILED:
		return filament.RunFailed, nil
	case ingestionv1.RunStatus_RUN_STATUS_CANCELED:
		return filament.RunCanceled, nil
	case ingestionv1.RunStatus_RUN_STATUS_PAUSED:
		return filament.RunPaused, nil
	case ingestionv1.RunStatus_RUN_STATUS_PARTIAL:
		return filament.RunPartial, nil
	case ingestionv1.RunStatus_RUN_STATUS_SCHEDULED:
		return filament.RunScheduled, nil
	default:
		return 0, fmt.Errorf("metrics: unsupported RunStatus %v", v)
	}
}

// keyToProtoStatus re-encodes a DIMENSION_STATUS group-by key — a domain
// RunStatus ordinal as text, straight off the runs.status column — back to
// the wire encoding, so a grouped response's keys mean the same thing a
// filter's values do. Non-numeric/unrecognized keys pass through unchanged
// (defensive; shouldn't happen for a key this store itself produced).
func keyToProtoStatus(key string) string {
	n, err := strconv.Atoi(key)
	if err != nil {
		return key
	}
	return strconv.Itoa(int(runStatusToProto(filament.RunStatus(n))))
}

// statusFilterValue is keyToProtoStatus's inverse: it converts one
// DIMENSION_STATUS filter value from its wire encoding (an
// ingestionv1.RunStatus ordinal, as the decimal string a client sends) to the
// domain RunStatus ordinal, as the decimal string filament.MetricsFilter.Values
// and the runs.status column both use.
func statusFilterValue(v string) (string, error) {
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return "", fmt.Errorf("metrics: status filter value %q is not a RunStatus number: %w", v, err)
	}
	status, err := runStatusFromProto(ingestionv1.RunStatus(n))
	if err != nil {
		return "", err
	}
	return strconv.Itoa(int(status)), nil
}

func metricFiltersFromProto(fs []*metricsv1.MetricFilter) ([]filament.MetricsFilter, error) {
	out := make([]filament.MetricsFilter, 0, len(fs))
	for _, f := range fs {
		dim := metricDimensionFromProto(f.GetDimension())
		if dim == filament.DimensionUnspecified || len(f.GetValues()) == 0 {
			continue
		}
		values := f.GetValues()
		if dim == filament.DimensionStatus {
			translated := make([]string, len(values))
			for i, v := range values {
				sv, err := statusFilterValue(v)
				if err != nil {
					return nil, err
				}
				translated[i] = sv
			}
			values = translated
		}
		out = append(out, filament.MetricsFilter{Dimension: dim, Values: values})
	}
	return out, nil
}

func metricGranularityFromProto(g metricsv1.MetricGranularity) (filament.MetricsGranularity, error) {
	switch g {
	case metricsv1.MetricGranularity_METRIC_GRANULARITY_HOUR:
		return filament.GranularityHour, nil
	case metricsv1.MetricGranularity_METRIC_GRANULARITY_DAY:
		return filament.GranularityDay, nil
	default:
		return 0, fmt.Errorf("metrics: granularity must be set")
	}
}

func untilOrNow(untilMs int64) time.Time {
	if untilMs == 0 {
		return time.Now()
	}
	return time.UnixMilli(untilMs)
}
