import { create } from "@bufbuild/protobuf";

import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/types";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";
import {
  Metric,
  MetricDimension,
  MetricFilterSchema,
  MetricGranularity,
  type QueryTimeseriesRequest,
  type Timeseries,
} from "@/gen/metrics/v1/metrics_pb";

import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createObservabilityTimeseriesInput,
  formatBucketKey,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/utils";
import {
  PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/pages/pipelines/history/constants";

export const createRunCountTimeseriesInput = (
  timeframe: ObservabilityTimeframe,
  statuses: RunStatus[],
): QueryTimeseriesRequest =>
  createObservabilityTimeseriesInput(timeframe, {
    metrics: [Metric.RUN_COUNT],
    groupBy: MetricDimension.STATUS,
    filters: [
      create(MetricFilterSchema, {
        dimension: MetricDimension.STATUS,
        values: statuses.map(String),
      }),
    ],
  });

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;

export const createScheduledRunsChartGroups = (
  runs: RunInfo[],
  timeframe: ObservabilityTimeframe,
): BarChartGroupDatum<ObservabilityRunMetric>[] => {
  const { durationMs, granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  const start = new Date();
  if (granularity === MetricGranularity.HOUR) {
    start.setMinutes(0, 0, 0);
  } else {
    start.setHours(0, 0, 0, 0);
  }
  const bucketCount = Math.round(
    durationMs / (granularity === MetricGranularity.HOUR ? HOUR_MS : DAY_MS),
  );
  const bucketBounds = Array.from({ length: bucketCount + 1 }, (_, index) =>
    granularity === MetricGranularity.HOUR
      ? start.getTime() + index * HOUR_MS
      : new Date(start.getFullYear(), start.getMonth(), start.getDate() + index).getTime(),
  );
  return bucketBounds.slice(0, -1).map((bucketStartMs, bucketIndex) => ({
    label: formatBucketKey(BigInt(bucketStartMs)),
    bars: [
      {
        metric: "runs" as const,
        components: [
          {
            key: String(RunStatus.SCHEDULED),
            label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED],
            value: runs.filter(
              (run) =>
                Number(run.scheduledAt) >= bucketStartMs &&
                Number(run.scheduledAt) < bucketBounds[bucketIndex + 1],
            ).length,
            color: PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP[RunStatus.SCHEDULED],
          },
        ],
      },
    ],
  }));
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
            color: PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP[status],
          };
        }),
      },
    ],
  }));
