import { match } from "ts-pattern";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import type { MetricGranularity, Timeseries } from "@/gen/metrics/v1/metrics_pb";

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
import {
  ObservabilityPipelineHealth,
  type ObservabilityTimeframe,
} from "@/pages/observability/types";
import {
  createTimeframeSince,
  formatBucketKey,
  getBucketLabelFormatter,
  getBucketOffset,
  getObservabilityBucketCount,
} from "@/pages/observability/utils";

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

const getObservabilityPipelinesCellStatus = (
  isRunning: boolean,
  failed: number,
  completed: number,
) => {
  if (isRunning) {
    return RunStatus.RUNNING;
  }
  if (failed > 0) {
    return RunStatus.FAILED;
  }
  if (completed > 0) {
    return RunStatus.COMPLETED;
  }
  return undefined;
};

const formatObservabilityPipelinesCellSummary = (
  { bucketStartMs, status, completed, failed }: Omit<ObservabilityPipelinesCell, "summary">,
  granularity: MetricGranularity,
) => {
  const parts = [
    status === RunStatus.RUNNING ? "running" : undefined,
    completed > 0 ? `${completed} completed` : undefined,
    failed > 0 ? `${failed} failed` : undefined,
  ].filter((part) => part !== undefined);
  const label = getBucketLabelFormatter(granularity)(formatBucketKey(bucketStartMs));
  return `${label} · ${parts.length > 0 ? parts.join(", ") : "No runs"}`;
};

export const createObservabilityPipelinesCells = (
  pipeline: Pipeline,
  health: ObservabilityPipelineHealth,
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

  return Array.from({ length: bucketCount }, (_, index) => {
    const cellStartMs = BigInt(getBucketOffset(granularity, since, index));
    const bucketKey = formatBucketKey(cellStartMs);
    const completedCount = completed?.get(bucketKey) ?? 0;
    const failedCount = failed?.get(bucketKey) ?? 0;
    const cell = {
      bucketStartMs: cellStartMs,
      status: getObservabilityPipelinesCellStatus(
        health === ObservabilityPipelineHealth.RUNNING && index === bucketCount - 1,
        failedCount,
        completedCount,
      ),
      completed: completedCount,
      failed: failedCount,
    };
    return { ...cell, summary: formatObservabilityPipelinesCellSummary(cell, granularity) };
  });
};

export const createObservabilityPipelinesBarSegments = ({
  status,
  completed,
  failed,
}: ObservabilityPipelinesCell): ObservabilityPipelinesBarSegment[] => {
  const total = completed + failed;
  if (total === 0) {
    return status === RunStatus.RUNNING
      ? [{ status: RunStatus.RUNNING, height: OBSERVABILITY_PIPELINES_BAR_HEIGHT }]
      : [];
  }
  const runningHeight =
    status === RunStatus.RUNNING ? OBSERVABILITY_PIPELINES_BAR_RUNNING_HEIGHT : 0;
  const height = OBSERVABILITY_PIPELINES_BAR_HEIGHT - runningHeight;
  const failedHeight =
    failed === 0
      ? 0
      : Math.min(
          completed === 0 ? height : height - 1,
          Math.max(1, Math.round((height * failed) / total)),
        );
  return [
    { status: RunStatus.RUNNING, height: runningHeight },
    { status: RunStatus.FAILED, height: failedHeight },
    { status: RunStatus.COMPLETED, height: height - failedHeight },
  ].filter((segment) => segment.height > 0);
};
