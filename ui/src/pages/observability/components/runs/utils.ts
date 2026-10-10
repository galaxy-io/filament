import { create } from "@bufbuild/protobuf";

import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/BarChart";
import type { ChartSelection } from "@galaxy-io/dls/charts/types";

import { type ListRunsRequest, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";
import {
  Metric,
  MetricDimension,
  MetricFilterSchema,
  type QueryTimeseriesRequest,
  type Timeseries,
} from "@/gen/metrics/v1/metrics_pb";

import {
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import {
  OBSERVABILITY_GRANULARITY_TO_BUCKET_OFFSET_MAP,
  OBSERVABILITY_GRANULARITY_TO_BUCKET_START_MAP,
  OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createObservabilityTimeseriesInput,
  createTimeframeSince,
  formatBucketKey,
} from "@/pages/observability/utils";

import { mapOptionIdToEnum } from "@/utils/select";

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

export const createScheduledRunsChartGroups = (
  runs: RunInfo[],
  timeframe: ObservabilityTimeframe,
): BarChartGroupDatum<ObservabilityRunMetric>[] => {
  const { durationMs, granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  const start = OBSERVABILITY_GRANULARITY_TO_BUCKET_START_MAP[granularity](new Date());
  const bucketCount = Math.round(
    durationMs / Number(OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP[granularity]),
  );
  const bucketBounds = Array.from({ length: bucketCount + 1 }, (_, index) =>
    OBSERVABILITY_GRANULARITY_TO_BUCKET_OFFSET_MAP[granularity](start, index),
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
            color: PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.SCHEDULED],
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
        components: series
          .flatMap((statusSeries) => {
            const status = mapOptionIdToEnum(RunStatus, statusSeries.key);
            const color = PIPELINE_RUN_STATUS_TO_HUE_MAP[status];
            return color
              ? [
                  {
                    key: statusSeries.key,
                    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
                    value: statusSeries.points[bucketIndex].values[0],
                    color,
                  },
                ]
              : [];
          })
          .sort((a, b) => b.value - a.value),
      },
    ],
  }));

export const mapChartSelectionToRunsFilter = (selection: ChartSelection | undefined) => {
  const status = mapOptionIdToEnum(RunStatus, selection?.seriesKey ?? "");
  return {
    runsBucket: selection?.categoryKey === undefined ? undefined : BigInt(selection.categoryKey),
    runsStatus: status === RunStatus.UNSPECIFIED ? undefined : status,
  };
};

export const createRunsWindowInput = (
  timeframe: ObservabilityTimeframe,
  runsBucket: bigint | undefined,
): Pick<ListRunsRequest, "sinceMs" | "untilMs"> => {
  const sinceMs = createTimeframeSince(timeframe);
  if (runsBucket === undefined) {
    return { sinceMs, untilMs: 0n };
  }
  const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  return {
    sinceMs: runsBucket > sinceMs ? runsBucket : sinceMs,
    untilMs: runsBucket + OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP[granularity],
  };
};
