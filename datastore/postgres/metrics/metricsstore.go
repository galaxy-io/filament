// Package metrics implements filament.MetricsStore over the same
// Postgres runs table datastore/postgres.Store persists to. It's a separate
// package/type — not a method set on Store itself — sharing the pool rather
// than growing the core datastore with dashboard-query-specific SQL.
//
// Which metrics/dimensions exist is a runtime-variable query shape (a
// caller-chosen subset of metrics, an optional group-by dimension, zero or
// more filters) that sqlc's static-query codegen can't express — the same
// reason datastore/postgres.Store.ListRuns is hand-rolled rather than sqlc-
// generated. Query text lives in templates/*.sql.tmpl (same embed idiom as
// datastore/postgres/migrate.go) rendered with pre-built, already-safe SQL
// fragments (registry.go's column/expression names, and $N placeholders from
// queryBuilder) — never raw values, which stay bound as query args.
package metrics

import (
	"context"
	"embed"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
)

//go:embed templates/*.sql.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.sql.tmpl"))

// Store answers run metrics queries against the runs table.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps an already-connected pool (see datastore/postgres.NewPool).
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var _ filament.MetricsStore = (*Store)(nil)

// Ping reports pool reachability, for readiness probes.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Close releases the pool. Implements io.Closer, checked optionally by
// callers (see cmd/server/main.go's DataStore cleanup for the same idiom).
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

func metricExprList(ms []filament.Metric) ([]string, error) {
	if len(ms) == 0 {
		return nil, fmt.Errorf("metrics: at least one metric is required")
	}
	exprs := make([]string, len(ms))
	for i, m := range ms {
		expr, err := metricExpr(m)
		if err != nil {
			return nil, err
		}
		exprs[i] = expr
	}
	return exprs, nil
}

func granularityUnit(g filament.MetricsGranularity) (string, error) {
	switch g {
	case filament.GranularityHour:
		return "hour", nil
	case filament.GranularityDay:
		return "day", nil
	default:
		return "", fmt.Errorf("metrics: granularity must be set")
	}
}

// queryBuilder accumulates $N-placeholdered args, mirroring
// datastore/postgres.Store.ListRuns's arg() closure.
type queryBuilder struct {
	args []any
}

func (b *queryBuilder) arg(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}

// whereClause builds "WHERE started_at >= $1 AND started_at < $2 [AND
// tenant_id = $n] [AND col IN ($n, ...)]*" — runs that never started are
// excluded, and an empty tenant omits the tenant filter entirely (matches
// every tenant), both matching ListRuns's own filter semantics
// (datastore/postgres.Store.ListRuns's "if f.Tenant != \"\"" guard).
func (b *queryBuilder) whereClause(tenant filament.TenantID, since, until time.Time, filters []filament.MetricsFilter) (string, error) {
	var s strings.Builder
	fmt.Fprintf(&s, "WHERE started_at >= %s AND started_at < %s", b.arg(since), b.arg(until))
	if tenant != "" {
		fmt.Fprintf(&s, " AND tenant_id = %s", b.arg(string(tenant)))
	}
	for _, f := range filters {
		col, ok := dimensionColumn(f.Dimension)
		if !ok || len(f.Values) == 0 {
			continue
		}
		placeholders := make([]string, len(f.Values))
		for i, v := range f.Values {
			if f.Dimension == filament.DimensionStatus {
				n, err := strconv.Atoi(v)
				if err != nil {
					return "", fmt.Errorf("metrics: status filter value %q is not a RunStatus number: %w", v, err)
				}
				placeholders[i] = b.arg(n)
			} else {
				placeholders[i] = b.arg(v)
			}
		}
		fmt.Fprintf(&s, " AND %s IN (%s)", col, strings.Join(placeholders, ", "))
	}
	return s.String(), nil
}

// keyExpr returns the SELECT expression for a query's group-by key: "”"
// (an ungrouped, single "" key) when dim is unset, else the dimension's
// column cast to text.
func keyExpr(dim filament.MetricsDimension) (expr, col string) {
	col, ok := dimensionColumn(dim)
	if !ok {
		return "''", ""
	}
	return col + "::text", col
}

func render(name string, data any) (string, error) {
	var b strings.Builder
	if err := templates.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("metrics: render %s: %w", name, err)
	}
	return b.String(), nil
}

// aggregateTemplateData fills templates/aggregate.sql.tmpl.
type aggregateTemplateData struct {
	KeyExpr, MetricList, Where, GroupCol string
}

// timeseriesTemplateData fills templates/timeseries.sql.tmpl.
type timeseriesTemplateData struct {
	KeyExpr, BucketExpr, Where, BucketFrom, BucketTo, Unit, AggSelectList, AggColList, GroupKeysExpr string
	// NoGroupBy adds a "" key so an ungrouped query still gets one series/row.
	NoGroupBy bool
}

// valuesOrZero converts nullable metric scan targets to a dense []float64,
// defaulting NULL (avg() over a group/bucket with no terminal runs) to 0.
func valuesOrZero(nullable []*float64) []float64 {
	values := make([]float64, len(nullable))
	for i, v := range nullable {
		if v != nil {
			values[i] = *v
		}
	}
	return values
}

// QueryRunAggregate returns one row per GroupBy value (a single "" key when
// GroupBy is unset) over [q.Since, q.Until).
func (s *Store) QueryRunAggregate(ctx context.Context, q filament.RunAggregateQuery) ([]filament.RunAggregateRow, error) {
	exprs, err := metricExprList(q.Metrics)
	if err != nil {
		return nil, err
	}
	b := &queryBuilder{}
	where, err := b.whereClause(q.Tenant, q.Since, q.Until, q.Filters)
	if err != nil {
		return nil, err
	}
	key, groupCol := keyExpr(q.GroupBy)

	query, err := render("aggregate.sql.tmpl", aggregateTemplateData{
		KeyExpr:    key,
		MetricList: strings.Join(exprs, ", "),
		Where:      where,
		GroupCol:   groupCol,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, query, b.args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: query run aggregate: %w", err)
	}
	defer rows.Close()

	var out []filament.RunAggregateRow
	for rows.Next() {
		var rowKey string
		// Nullable scan targets: avg(duration) is NULL when a group has no
		// terminal runs (unlike the sum metrics, which are coalesced to 0 in
		// the registry expression) — nil defaults to 0 in the returned row.
		nullable := make([]*float64, len(q.Metrics))
		scanArgs := append([]any{&rowKey}, valuePtrs(nullable)...)
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("metrics: scan run aggregate row: %w", err)
		}
		out = append(out, filament.RunAggregateRow{Key: rowKey, Values: valuesOrZero(nullable)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: run aggregate rows: %w", err)
	}
	return out, nil
}

// groupKeys returns the ordered set of keys a covering filter requires the
// response to contain (metrics.proto's Timeseries doc: "one series per
// requested value ... in request-value order"), or nil when no filter covers
// GroupBy, falling back to whatever keys are observed in the data.
func groupKeys(q filament.RunTimeseriesQuery) []string {
	if q.GroupBy == filament.DimensionUnspecified {
		return nil
	}
	for _, f := range q.Filters {
		if f.Dimension == q.GroupBy {
			return f.Values
		}
	}
	return nil
}

// bucketExprs returns the SQL expression for a bucket's own boundary (expr)
// and the query range's start/end (from/to), all shifted into
// tzOffsetMinutes-local time and truncated to unit so generate_series's range
// (from/to) lines up exactly with individual buckets (expr). to backs off one
// microsecond before truncating so a range whose end lands exactly on a
// boundary doesn't pull in an extra, out-of-range bucket (until is
// exclusive).
func (b *queryBuilder) bucketExprs(since, until time.Time, tzOffsetMinutes int, unit string) (expr, from, to string) {
	tz := fmt.Sprintf("(%s * interval '1 minute')", b.arg(tzOffsetMinutes))
	expr = fmt.Sprintf("date_trunc('%s', started_at + %s) - %s", unit, tz, tz)
	from = fmt.Sprintf("date_trunc('%s', %s::timestamptz + %s) - %s", unit, b.arg(since), tz, tz)
	to = fmt.Sprintf("date_trunc('%s', (%s::timestamptz - interval '1 microsecond') + %s) - %s", unit, b.arg(until), tz, tz)
	return expr, from, to
}

// aggSelectCols builds a timeseries query's per-metric SELECT aliases (m0,
// m1, ...) and the agg.m* column references that pull them back out after
// the bucket/key spine LEFT JOINs against them.
func aggSelectCols(exprs []string) (selectList, cols []string) {
	selectList = make([]string, len(exprs))
	cols = make([]string, len(exprs))
	for i, e := range exprs {
		selectList[i] = fmt.Sprintf("%s AS m%d", e, i)
		cols[i] = fmt.Sprintf("agg.m%d", i)
	}
	return selectList, cols
}

// QueryRunTimeseries returns one dense, zero-filled, ascending series per
// GroupBy value (a single "" series when GroupBy is unset) bucketed at
// q.Granularity over [q.Since, q.Until). When a filter covers GroupBy, the
// filter's values (not the data) determine which series exist and their
// order, so a requested value with no matching runs still comes back
// zero-filled rather than missing (see groupKeys).
func (s *Store) QueryRunTimeseries(ctx context.Context, q filament.RunTimeseriesQuery) ([]filament.RunTimeseries, error) {
	exprs, err := metricExprList(q.Metrics)
	if err != nil {
		return nil, err
	}
	unit, err := granularityUnit(q.Granularity)
	if err != nil {
		return nil, err
	}
	b := &queryBuilder{}
	where, err := b.whereClause(q.Tenant, q.Since, q.Until, q.Filters)
	if err != nil {
		return nil, err
	}
	key, groupCol := keyExpr(q.GroupBy)
	bucketExpr, bucketFrom, bucketTo := b.bucketExprs(q.Since, q.Until, q.TZOffsetMinutes, unit)
	aggSelect, aggCols := aggSelectCols(exprs)

	var groupKeysExpr string
	if keys := groupKeys(q); len(keys) > 0 {
		groupKeysExpr = b.arg(keys) + "::text[]"
	}

	query, err := render("timeseries.sql.tmpl", timeseriesTemplateData{
		KeyExpr:       key,
		BucketExpr:    bucketExpr,
		Where:         where,
		BucketFrom:    bucketFrom,
		BucketTo:      bucketTo,
		Unit:          unit,
		AggSelectList: strings.Join(aggSelect, ", "),
		AggColList:    strings.Join(aggCols, ", "),
		GroupKeysExpr: groupKeysExpr,
		NoGroupBy:     groupCol == "",
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, query, b.args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: query run timeseries: %w", err)
	}
	defer rows.Close()

	return scanTimeseries(rows, len(q.Metrics))
}

// scanTimeseries reads rows shaped (key, bucket, m0, m1, ...) — dense per
// templates/timeseries.sql.tmpl — into one filament.RunTimeseries per
// distinct key, preserving the SQL's own row order (see groupKeys for what
// determines that order).
func scanTimeseries(rows pgx.Rows, numMetrics int) ([]filament.RunTimeseries, error) {
	series := map[string]*filament.RunTimeseries{}
	var order []string
	for rows.Next() {
		var rowKey string
		var bucket time.Time
		values := make([]*float64, numMetrics)
		scanArgs := append([]any{&rowKey, &bucket}, valuePtrs(values)...)
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("metrics: scan run timeseries row: %w", err)
		}
		ts, ok := series[rowKey]
		if !ok {
			ts = &filament.RunTimeseries{Key: rowKey}
			series[rowKey] = ts
			order = append(order, rowKey)
		}
		ts.Points = append(ts.Points, filament.TimeseriesPoint{BucketStart: bucket, Values: valuesOrZero(values)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: run timeseries rows: %w", err)
	}

	out := make([]filament.RunTimeseries, 0, len(order))
	for _, k := range order {
		out = append(out, *series[k])
	}
	return out, nil
}

// valuePtrs returns scan targets for nullable metric columns: avg() is NULL
// when a group/bucket has no terminal runs (see QueryRunAggregate/
// QueryRunTimeseries, which default a nil back to 0 in the returned row).
func valuePtrs(values []*float64) []any {
	ptrs := make([]any, len(values))
	for i := range values {
		ptrs[i] = &values[i]
	}
	return ptrs
}
