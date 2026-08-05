package postgres

import (
	"fmt"

	"github.com/galaxy-io/filament"
)

// dimensionColumns maps a MetricsDimension to its runs column — the single
// source of truth for every dimension this store supports. Add a new
// dimension here (plus the matching proto and filament.MetricsDimension enum
// value) rather than touching query logic.
var dimensionColumns = map[filament.MetricsDimension]string{
	filament.DimensionTenantID:   "tenant_id",
	filament.DimensionPipelineID: "(request->>'PipelineID')",
	filament.DimensionStatus:     "status",
}

// metricExprs maps a Metric to its SQL aggregate expression over runs — the
// single source of truth for every metric this store supports. Duration
// averages extract(epoch from (finished_at - started_at)); Postgres's avg()
// ignores NULLs, and finished_at is only set on terminal runs, so this is
// naturally "mean duration over terminal runs" with no extra filtering.
var metricExprs = map[filament.Metric]string{
	filament.MetricRunCount:    "count(*)",
	filament.MetricRunRecords:  "coalesce(sum(records), 0)",
	filament.MetricRunBytes:    "coalesce(sum(bytes), 0)",
	filament.MetricRunDuration: "avg(extract(epoch from (finished_at - started_at)) * 1000)",
}

func dimensionColumn(d filament.MetricsDimension) (string, bool) {
	col, ok := dimensionColumns[d]
	return col, ok
}

func metricExpr(m filament.Metric) (string, error) {
	expr, ok := metricExprs[m]
	if !ok {
		return "", fmt.Errorf("metricsstore/postgres: unsupported metric %v", m)
	}
	return expr, nil
}
