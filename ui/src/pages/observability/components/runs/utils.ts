import { create } from "@bufbuild/protobuf";

import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/types";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import {
  Metric,
  MetricDimension,
  MetricFilterSchema,
  type QueryTimeseriesRequest,
  QueryTimeseriesRequestSchema,
  type Timeseries,
} from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP } from "@/pages/observability/components/runs/constants";
import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createTimeframeSince,
  formatBucketKey,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/utils";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";

export const createRunCountTimeseriesInput = (
  timeframe: ObservabilityTimeframe,
  statuses: RunStatus[],
): QueryTimeseriesRequest => {
  const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  return create(QueryTimeseriesRequestSchema, {
    metrics: [Metric.RUN_COUNT],
    sinceMs: createTimeframeSince(timeframe),
    granularity,
    tzOffsetMinutes: -new Date().getTimezoneOffset(),
    groupBy: MetricDimension.STATUS,
    filters: [
      create(MetricFilterSchema, {
        dimension: MetricDimension.STATUS,
        values: statuses.map(String),
      }),
    ],
  });
};

export const mapTimeseriesToChartGroups = (
  series: Timeseries[],
): BarChartGroupDatum<ObservabilityRunMetric>[] =>
  (series[0]?.points ?? []).map((point, bucketIndex) => ({
    label: formatBucketKey(point.bucketStartMs),
    bars: [
      {
        metric: "runs",
        components: series.map((statusSeries) => {
          const status = Number(statusSeries.key) as RunStatus;
          return {
            key: statusSeries.key,
            label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
            value: statusSeries.points[bucketIndex].values[0],
            color: OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP[status],
          };
        }),
      },
    ],
  }));
