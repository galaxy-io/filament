import { match } from "ts-pattern";

import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/BarChart";
import type { LineChartLineDatum } from "@galaxy-io/dls/charts/LineChart";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";
import { formatNumber } from "@galaxy-io/dls/utils/format";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { MetricDimension, type Timeseries } from "@/gen/metrics/v1/metrics_pb";

import { formatPipelineName } from "@/components/pipelines/utils";
import {
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

import { OBSERVABILITY_BAND_PIPELINE_COLORS } from "@/pages/observability/components/band/constants";
import type {
  ObservabilityBandRunsMetric,
  ObservabilityBandSeriesEntry,
} from "@/pages/observability/components/band/types";
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  formatBucketKey,
  getBucketOffset,
  getObservabilityBucketCount,
} from "@/pages/observability/utils";

import { mapOptionIdToEnum } from "@/utils/select";

export const formatObservabilityBandCount = (value: number) =>
  formatNumber(value, { precision: 0 });

export const mapObservabilityBandRunsGroups = (
  series: Timeseries[],
): BarChartGroupDatum<ObservabilityBandRunsMetric>[] =>
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

export const getObservabilityBandUpcomingBounds = (
  timeframe: ObservabilityTimeframe,
  bucketStartMs: number,
) => {
  const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  return Array.from({ length: getObservabilityBucketCount(timeframe) + 1 }, (_, index) =>
    getBucketOffset(granularity, new Date(bucketStartMs), index),
  );
};

const isObservabilityBandRunWithin = (run: RunInfo, startMs: number, endMs: number) =>
  Number(run.scheduledAt) >= startMs && Number(run.scheduledAt) < endMs;

export const filterObservabilityBandUpcomingRuns = (runs: RunInfo[], bounds: number[]) =>
  runs.filter((run) => isObservabilityBandRunWithin(run, bounds[0], bounds[bounds.length - 1]));

export const mapObservabilityBandUpcomingGroups = (
  runs: RunInfo[],
  bounds: number[],
): BarChartGroupDatum<ObservabilityBandRunsMetric>[] =>
  bounds.slice(0, -1).map((startMs, bucketIndex) => ({
    label: formatBucketKey(BigInt(startMs)),
    bars: [
      {
        metric: "runs",
        components: [
          {
            key: String(RunStatus.SCHEDULED),
            label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED],
            value: runs.filter((run) =>
              isObservabilityBandRunWithin(run, startMs, bounds[bucketIndex + 1]),
            ).length,
            color: PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.SCHEDULED],
          },
        ],
      },
    ],
  }));

const getObservabilityBandPipelineEntries = (
  series: Timeseries[],
  ranking: { pipeline: Pipeline }[],
): ObservabilityBandSeriesEntry[] => {
  const seriesByPipelineId = new Map(series.map((keySeries) => [keySeries.key, keySeries]));
  return ranking.flatMap(({ pipeline }, index) => {
    const keySeries = seriesByPipelineId.get(pipeline.id);
    return keySeries
      ? [
          {
            key: pipeline.id,
            label: formatPipelineName(pipeline, true),
            color:
              OBSERVABILITY_BAND_PIPELINE_COLORS[index % OBSERVABILITY_BAND_PIPELINE_COLORS.length],
            points: keySeries.points,
          },
        ]
      : [];
  });
};

const getObservabilityBandStatusEntries = (series: Timeseries[]): ObservabilityBandSeriesEntry[] =>
  series.flatMap(({ key, points }) => {
    const status = mapOptionIdToEnum(RunStatus, key);
    const color = PIPELINE_RUN_STATUS_TO_HUE_MAP[status];
    return color ? [{ key, label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status], color, points }] : [];
  });

export const mapObservabilityBandLines = (
  pivot: MetricDimension,
  series: Timeseries[],
  ranking: { pipeline: Pipeline }[],
  label: string,
  color: RoleColor,
) => {
  const entries = match(pivot)
    .with(MetricDimension.PIPELINE_ID, () => getObservabilityBandPipelineEntries(series, ranking))
    .with(MetricDimension.STATUS, () => getObservabilityBandStatusEntries(series))
    .otherwise(() => [{ key: label, label, color, points: series[0]?.points ?? [] }]);
  return {
    series: Object.fromEntries(
      entries.map((entry) => [entry.key, { label: entry.label, color: entry.color }]),
    ),
    lines: entries.map<LineChartLineDatum<string>>(({ key, points }) => ({
      metric: key,
      showArea: pivot === MetricDimension.UNSPECIFIED,
      points: points.map((point) => ({
        x: formatBucketKey(point.bucketStartMs),
        y: point.values[0],
      })),
    })),
  };
};

export const getObservabilityBandTopColors = (pivot: MetricDimension, color: RoleColor) =>
  pivot === MetricDimension.PIPELINE_ID ? OBSERVABILITY_BAND_PIPELINE_COLORS : [color];
