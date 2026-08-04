// Package metrics implements MetricsService by converting proto requests to
// filament.MetricsStore queries and back. Mirrors server/: the Connect
// handler lives here, cmd/metrics only wires it up.
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	metricsv1 "github.com/galaxy-io/filament/api/metrics/v1"
	"github.com/galaxy-io/filament/api/metrics/v1/metricsv1connect"
)

// Server implements MetricsService over a filament.MetricsStore.
type Server struct {
	store filament.MetricsStore
}

// New returns a Server querying store (see datastore/postgres/metricsstore).
func New(store filament.MetricsStore) *Server { return &Server{store: store} }

// Mount registers the Connect handler on mux.
func (s *Server) Mount(mux *http.ServeMux) {
	path, handler := metricsv1connect.NewMetricsServiceHandler(s)
	mux.Handle(path, handler)
}

var _ metricsv1connect.MetricsServiceHandler = (*Server)(nil)

func toMetric(m metricsv1.Metric) (filament.Metric, error) {
	switch m {
	case metricsv1.Metric_METRIC_RUN_COUNT:
		return filament.MetricRunCount, nil
	case metricsv1.Metric_METRIC_RUN_RECORDS:
		return filament.MetricRunRecords, nil
	case metricsv1.Metric_METRIC_RUN_BYTES:
		return filament.MetricRunBytes, nil
	case metricsv1.Metric_METRIC_RUN_DURATION:
		return filament.MetricRunDuration, nil
	default:
		return 0, fmt.Errorf("metrics: unsupported metric %v", m)
	}
}

func toMetrics(ms []metricsv1.Metric) ([]filament.Metric, error) {
	if len(ms) == 0 {
		return nil, fmt.Errorf("metrics: at least one metric is required")
	}
	out := make([]filament.Metric, len(ms))
	for i, m := range ms {
		fm, err := toMetric(m)
		if err != nil {
			return nil, err
		}
		out[i] = fm
	}
	return out, nil
}

func toDimension(d metricsv1.Dimension) filament.MetricsDimension {
	switch d {
	case metricsv1.Dimension_DIMENSION_TENANT_ID:
		return filament.DimensionTenantID
	case metricsv1.Dimension_DIMENSION_PIPELINE_ID:
		return filament.DimensionPipelineID
	case metricsv1.Dimension_DIMENSION_STATUS:
		return filament.DimensionStatus
	default:
		return filament.DimensionUnspecified
	}
}

// statusFromProto/statusToProto translate DIMENSION_STATUS values between the
// wire encoding (ingestion.v1.RunStatus — the only RunStatus a client has,
// same as ListRunsRequest.status) and filament.RunStatus's domain ordinal,
// which is what's actually stored in the runs.status column. The two don't
// numerically align: the proto enum reserves 0 for RUN_STATUS_UNSPECIFIED, so
// every real value is shifted by one relative to the domain iota. Mirrors
// server/convert.go's runStatusToProto/runStatusesFromProto for IngestionService.
func statusFromProto(v ingestionv1.RunStatus) (filament.RunStatus, error) {
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
	default:
		return 0, fmt.Errorf("metrics: unsupported RunStatus %v", v)
	}
}

func statusToProto(s filament.RunStatus) ingestionv1.RunStatus {
	switch s {
	case filament.RunRequested:
		return ingestionv1.RunStatus_RUN_STATUS_REQUESTED
	case filament.RunRunning:
		return ingestionv1.RunStatus_RUN_STATUS_RUNNING
	case filament.RunCompleted:
		return ingestionv1.RunStatus_RUN_STATUS_COMPLETED
	case filament.RunFailed:
		return ingestionv1.RunStatus_RUN_STATUS_FAILED
	case filament.RunCanceled:
		return ingestionv1.RunStatus_RUN_STATUS_CANCELED
	case filament.RunPaused:
		return ingestionv1.RunStatus_RUN_STATUS_PAUSED
	case filament.RunPartial:
		return ingestionv1.RunStatus_RUN_STATUS_PARTIAL
	default:
		return ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED
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
	return strconv.Itoa(int(statusToProto(filament.RunStatus(n))))
}

func toFilters(fs []*metricsv1.MetricFilter) ([]filament.MetricsFilter, error) {
	out := make([]filament.MetricsFilter, 0, len(fs))
	for _, f := range fs {
		dim := toDimension(f.GetDimension())
		if dim == filament.DimensionUnspecified || len(f.GetValues()) == 0 {
			continue
		}
		values := f.GetValues()
		if dim == filament.DimensionStatus {
			translated := make([]string, len(values))
			for i, v := range values {
				n, err := strconv.ParseInt(v, 10, 32)
				if err != nil {
					return nil, fmt.Errorf("metrics: status filter value %q is not a RunStatus number: %w", v, err)
				}
				status, err := statusFromProto(ingestionv1.RunStatus(n))
				if err != nil {
					return nil, err
				}
				translated[i] = strconv.Itoa(int(status))
			}
			values = translated
		}
		out = append(out, filament.MetricsFilter{Dimension: dim, Values: values})
	}
	return out, nil
}

func toGranularity(g metricsv1.MetricsGranularity) (filament.MetricsGranularity, error) {
	switch g {
	case metricsv1.MetricsGranularity_METRICS_GRANULARITY_HOUR:
		return filament.GranularityHour, nil
	case metricsv1.MetricsGranularity_METRICS_GRANULARITY_DAY:
		return filament.GranularityDay, nil
	default:
		return 0, fmt.Errorf("metrics: granularity must be set")
	}
}

// tenantFromRequest resolves the calling tenant. MetricsService has no
// tenant_id request field yet (unlike ListRunsRequest); DIMENSION_TENANT_ID
// filters/group_by still work against whatever the store scopes queries to.
// TODO: thread a real tenant once auth/session context exists.
func tenantFromRequest() filament.TenantID { return "" }

// QueryAggregate returns one row per group_by value (a single "" key when
// group_by is unset) over [since_ms, until_ms).
func (s *Server) QueryAggregate(ctx context.Context, req *connect.Request[metricsv1.QueryAggregateRequest]) (*connect.Response[metricsv1.QueryAggregateResponse], error) {
	m := req.Msg
	metrics, err := toMetrics(m.GetMetrics())
	if err != nil {
		return nil, err
	}
	filters, err := toFilters(m.GetFilters())
	if err != nil {
		return nil, err
	}
	groupBy := toDimension(m.GetGroupBy())

	rows, err := s.store.QueryRunAggregate(ctx, filament.RunAggregateQuery{
		Tenant:  tenantFromRequest(),
		Metrics: metrics,
		Since:   time.UnixMilli(m.GetSinceMs()),
		Until:   time.UnixMilli(m.GetUntilMs()),
		GroupBy: groupBy,
		Filters: filters,
	})
	if err != nil {
		return nil, err
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
func (s *Server) QueryTimeseries(ctx context.Context, req *connect.Request[metricsv1.QueryTimeseriesRequest]) (*connect.Response[metricsv1.QueryTimeseriesResponse], error) {
	m := req.Msg
	metrics, err := toMetrics(m.GetMetrics())
	if err != nil {
		return nil, err
	}
	granularity, err := toGranularity(m.GetGranularity())
	if err != nil {
		return nil, err
	}
	filters, err := toFilters(m.GetFilters())
	if err != nil {
		return nil, err
	}
	groupBy := toDimension(m.GetGroupBy())

	series, err := s.store.QueryRunTimeseries(ctx, filament.RunTimeseriesQuery{
		Tenant:          tenantFromRequest(),
		Metrics:         metrics,
		Since:           time.UnixMilli(m.GetSinceMs()),
		Until:           time.UnixMilli(m.GetUntilMs()),
		Granularity:     granularity,
		TZOffsetMinutes: int(m.GetTzOffsetMinutes()),
		GroupBy:         groupBy,
		Filters:         filters,
	})
	if err != nil {
		return nil, err
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
