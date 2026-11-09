import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/BarChart";
import type { LineChartLineDatum } from "@galaxy-io/dls/charts/LineChart";
import {
  EMPTY_VALUE,
  formatBytes,
  formatDuration,
  formatNumber,
} from "@galaxy-io/dls/utils/format";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import {
  Metric,
  MetricDimension,
  type QueryTimeseriesRequest,
  type Timeseries,
} from "@/gen/metrics/v1/metrics_pb";

import { formatPipelineName } from "@/components/pipelines/utils";
import {
  PIPELINE_RUN_EXECUTED_STATUSES,
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

import {
  OBSERVABILITY_BAND_PIPELINE_COLORS,
  OBSERVABILITY_BAND_TOTAL_METRICS,
} from "@/pages/observability/components/band/constants";
import type { ObservabilityBandRunsMetric } from "@/pages/observability/components/band/types";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createObservabilityStatusFilter,
  createObservabilityTimeseriesInput,
  formatBucketKey,
} from "@/pages/observability/utils";

import { mapOptionIdToEnum } from "@/utils/select";

export const formatObservabilityBandCount = (value: number) =>
  formatNumber(value, { precision: 0 });

export const formatObservabilityBandCpuAxis = (value: number) => formatDuration(value * 1_000);

export const formatObservabilityBandCpu = (value: number) =>
  value > 0 ? formatDuration(value * 1_000) : EMPTY_VALUE;

export const formatObservabilityBandMemory = (value: number) =>
  value > 0 ? formatBytes(value) : EMPTY_VALUE;

export const createObservabilityBandRunsInput = (
  timeframe: ObservabilityTimeframe,
  bucketStartMs: number,
): QueryTimeseriesRequest =>
  createObservabilityTimeseriesInput(timeframe, bucketStartMs, {
    metrics: [Metric.RUN_COUNT],
    groupBy: MetricDimension.STATUS,
    filters: [createObservabilityStatusFilter(PIPELINE_RUN_EXECUTED_STATUSES)],
  });

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

export const getObservabilityBandTotal = (values: number[], metric: Metric) =>
  values[OBSERVABILITY_BAND_TOTAL_METRICS.indexOf(metric)] ?? 0;

export const mapObservabilityBandPipelineLines = (
  ranking: { pipeline: Pipeline }[],
  series: Timeseries[],
) => {
  const seriesByPipelineId = new Map(series.map((keySeries) => [keySeries.key, keySeries]));
  const ranked = ranking.flatMap(({ pipeline }, index) => {
    const keySeries = seriesByPipelineId.get(pipeline.id);
    return keySeries
      ? [
          {
            pipeline,
            keySeries,
            color:
              OBSERVABILITY_BAND_PIPELINE_COLORS[index % OBSERVABILITY_BAND_PIPELINE_COLORS.length],
          },
        ]
      : [];
  });
  return {
    series: Object.fromEntries(
      ranked.map(({ pipeline, color }) => [
        pipeline.id,
        { label: formatPipelineName(pipeline, true), color },
      ]),
    ),
    lines: ranked.map<LineChartLineDatum<string>>(({ pipeline, keySeries }) => ({
      metric: pipeline.id,
      points: keySeries.points.map((point) => ({
        x: formatBucketKey(point.bucketStartMs),
        y: point.values[0],
      })),
    })),
  };
};
