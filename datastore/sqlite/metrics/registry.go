package metrics

import (
	"fmt"

	"github.com/galaxy-io/filament"
)

// dimensionColumns maps a MetricsDimension to its runs column — the single
// source of truth for every dimension this store supports, mirroring
// datastore/postgres/metrics.
var dimensionColumns = map[filament.MetricsDimension]string{
	filament.DimensionTenantID:   "tenant_id",
	filament.DimensionPipelineID: "pipeline_id",
	filament.DimensionStatus:     "status",
}

// metricExprs maps a Metric to its SQL aggregate expression over runs.
// Times are unix milliseconds, so duration is a plain difference; the usage
// columns route zero ("no usage reported") through avg()-ignores-NULLs via
// nullif, same as the postgres store.
var metricExprs = map[filament.Metric]string{
	filament.MetricRunCount:       "count(*)",
	filament.MetricRunRecords:     "coalesce(sum(records), 0)",
	filament.MetricRunBytes:       "coalesce(sum(bytes), 0)",
	filament.MetricRunDuration:    "avg(ended_at - started_at)",
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
