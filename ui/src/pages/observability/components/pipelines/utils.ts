import { match } from "ts-pattern";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import type { MetricGranularity, Timeseries } from "@/gen/metrics/v1/metrics_pb";

import { PIPELINE_RUN_STATUS_TO_HUE_MAP } from "@/components/runs/constants";
import { getPipelineRunStatusHue, getPipelineRunStatusLabel } from "@/components/runs/utils";

import {
  OBSERVABILITY_PIPELINES_BAR_GAP,
  OBSERVABILITY_PIPELINES_BAR_HEIGHT,
  OBSERVABILITY_PIPELINES_BAR_RUNNING_HEIGHT,
  OBSERVABILITY_PIPELINES_BAR_WIDTH,
  OBSERVABILITY_PIPELINES_COLUMN_TO_LABEL_MAP,
} from "@/pages/observability/components/pipelines/constants";
import {
  type ObservabilityPipelinesBarSegment,
  type ObservabilityPipelinesCell,
  ObservabilityPipelinesColumn,
  type ObservabilityPipelinesCounts,
} from "@/pages/observability/components/pipelines/types";
import {
  OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createTimeframeSince,
  formatBucketKey,
  getBucketLabelFormatter,
  getBucketOffset,
  getObservabilityBucketCount,
} from "@/pages/observability/utils";

import { isRunActive } from "@/utils/runs";

export const getObservabilityPipelinesTrackWidth = (bucketCount: number) =>
  bucketCount * OBSERVABILITY_PIPELINES_BAR_WIDTH +
  (bucketCount - 1) * OBSERVABILITY_PIPELINES_BAR_GAP;

export const getObservabilityPipelinesColumnLabel = (
  column: ObservabilityPipelinesColumn,
  timeframe: ObservabilityTimeframe,
) =>
  match(column)
    .with(
      ObservabilityPipelinesColumn.RUNS,
      () => `Last ${OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP[timeframe]}`,
    )
    .otherwise(() => OBSERVABILITY_PIPELINES_COLUMN_TO_LABEL_MAP[column]);

export const mapObservabilityPipelinesSeries = (
  series: Timeseries[],
): ObservabilityPipelinesCounts =>
  new Map(
    series.map(({ key, points }) => [
      key,
      new Map(points.map((point) => [formatBucketKey(point.bucketStartMs), point.values[0] ?? 0])),
    ]),
  );

const formatObservabilityPipelinesCellSummary = (
  { bucketStartMs, liveRun, completed, failed }: Omit<ObservabilityPipelinesCell, "summary">,
  granularity: MetricGranularity,
) => {
  const parts = [
    liveRun
      ? getPipelineRunStatusLabel(
          liveRun.status,
          liveRun.failedResources,
          liveRun.executionStatus,
        ).toLowerCase()
      : undefined,
    completed > 0 ? `${completed} completed` : undefined,
    failed > 0 ? `${failed} failed` : undefined,
  ].filter((part) => part !== undefined);
  const label = getBucketLabelFormatter(granularity)(formatBucketKey(bucketStartMs));
  return `${label} · ${parts.length > 0 ? parts.join(", ") : "No runs"}`;
};

export const createObservabilityPipelinesCells = (
  pipeline: Pipeline,
  timeframe: ObservabilityTimeframe,
  bucketStartMs: number,
  completedCounts: ObservabilityPipelinesCounts,
  failedCounts: ObservabilityPipelinesCounts,
): ObservabilityPipelinesCell[] => {
  const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  const completed = completedCounts.get(pipeline.id);
  const failed = failedCounts.get(pipeline.id);
  const since = new Date(Number(createTimeframeSince(timeframe, bucketStartMs)));
  const bucketCount = getObservabilityBucketCount(timeframe);
  const { lastRun } = pipeline;
  const activeRun = lastRun && isRunActive(lastRun) ? lastRun : undefined;

  return Array.from({ length: bucketCount }, (_, index) => {
    const cellStartMs = BigInt(getBucketOffset(granularity, since, index));
    const bucketKey = formatBucketKey(cellStartMs);
    const completedCount = completed?.get(bucketKey) ?? 0;
    const failedCount = failed?.get(bucketKey) ?? 0;
    const cell = {
      bucketStartMs: cellStartMs,
      liveRun: index === bucketCount - 1 ? activeRun : undefined,
      completed: completedCount,
      failed: failedCount,
    };
    return { ...cell, summary: formatObservabilityPipelinesCellSummary(cell, granularity) };
  });
};

export const createObservabilityPipelinesBarSegments = ({
  liveRun,
  completed,
  failed,
}: ObservabilityPipelinesCell): ObservabilityPipelinesBarSegment[] => {
  const total = completed + failed;
  const liveHue = liveRun
    ? getPipelineRunStatusHue(liveRun.status, liveRun.failedResources, liveRun.executionStatus)
    : undefined;
  if (total === 0) {
    return liveRun
      ? [{ key: "live", hue: liveHue, height: OBSERVABILITY_PIPELINES_BAR_HEIGHT }]
      : [];
  }
  const liveHeight = liveRun ? OBSERVABILITY_PIPELINES_BAR_RUNNING_HEIGHT : 0;
  const height = OBSERVABILITY_PIPELINES_BAR_HEIGHT - liveHeight;
  const failedHeight =
    failed === 0
      ? 0
      : Math.min(
          completed === 0 ? height : height - 1,
          Math.max(1, Math.round((height * failed) / total)),
        );
  return [
    { key: "live", hue: liveHue, height: liveHeight },
    {
      key: "failed",
      hue: PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.FAILED],
      height: failedHeight,
    },
    {
      key: "completed",
      hue: PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.COMPLETED],
      height: height - failedHeight,
    },
  ].filter((segment) => segment.height > 0);
};
