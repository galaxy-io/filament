package metrics

import (
	"fmt"

	"github.com/galaxy-io/filament"
)

// dimensionColumns maps a MetricsDimension to its runs column — the single
// source of truth for every dimension this store supports. Add a new
// dimension here (plus the matching proto and filament.MetricsDimension enum
// value) rather than touching query logic.
//
// Every entry must be a real column, never a JSON path. runs_metrics_idx
// covers the tenant, pipeline, version, status, and start-time dimensions.
var dimensionColumns = map[filament.MetricsDimension]string{
	filament.DimensionTenantID:   "tenant_id",
	filament.DimensionPipelineID: "pipeline_id",
	filament.DimensionStatus:     "status",
}

// metricExprs maps a Metric to its SQL aggregate expression over runs — the
// single source of truth for every metric this store supports. Duration
// averages extract(epoch from (ended_at - started_at)); Postgres's avg()
// ignores NULLs, and ended_at is only set on terminal runs, so this is
// naturally "mean duration over terminal runs" with no extra filtering.
// The usage columns are NOT NULL DEFAULT 0 and zero means "no usage
// reported" (runs outside a cgroup never heartbeat), so
// nullif routes those rows through the same avg()-ignores-NULLs path.
//
// Columns referenced here must also appear in the filtered CTE's select list
// in templates/timeseries.sql.tmpl — the aggregate template selects straight
// from runs, so an omission fails only on the timeseries path, at query time.
var metricExprs = map[filament.Metric]string{
	filament.MetricRunCount:       "count(*)",
	filament.MetricRunRecords:     "coalesce(sum(records), 0)",
	filament.MetricRunBytes:       "coalesce(sum(bytes), 0)",
	filament.MetricRunDuration:    "avg(extract(epoch from (ended_at - started_at)) * 1000)",
	filament.MetricRunMemoryUsage: "avg(nullif(memory_peak_bytes, 0))",
	filament.MetricRunCPUUsage:    "avg(nullif(cpu_seconds, 0))",
}

func dimensionColumn(d filament.MetricsDimension) (string, bool) {
	col, ok := dimensionColumns[d]
	return col, ok
}

func metricExpr(m filament.Metric) (string, error) {
	expr, ok := metricExprs[m]
	if !ok {
		return "", fmt.Errorf("metrics: unsupported metric %v", m)
	}
	return expr, nil
}
