import { create } from "@bufbuild/protobuf";
import { match } from "ts-pattern";

import {
  type Metric,
  type MetricDimension,
  type MetricFilter,
  MetricGranularity,
  type QueryTimeseriesRequest,
  QueryTimeseriesRequestSchema,
  type TimeseriesPoint,
} from "@/gen/metrics/v1/metrics_pb";

import {
  OBSERVABILITY_DAY_BUCKET_MIDPOINT_MS,
  OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP,
  OBSERVABILITY_HOUR_MS,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";

export const formatBucketKey = (bucketStartMs: TimeseriesPoint["bucketStartMs"]) =>
  bucketStartMs.toString();

const formatHourBucketLabel = (bucketKey: string) =>
  `${new Date(Number(bucketKey)).getHours().toString().padStart(2, "0")}:00`;

const formatDayBucketLabel = (bucketKey: string) => {
  const date = new Date(Number(bucketKey) + OBSERVABILITY_DAY_BUCKET_MIDPOINT_MS);
  return `${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")}`;
};

export const getBucketLabelFormatter = (granularity: MetricGranularity) =>
  match(granularity)
    .with(MetricGranularity.DAY, () => formatDayBucketLabel)
    .otherwise(() => formatHourBucketLabel);

export const getBucketStart = (granularity: MetricGranularity, date: Date) =>
  match(granularity)
    .with(
      MetricGranularity.DAY,
      () => new Date(date.getFullYear(), date.getMonth(), date.getDate()),
    )
    .otherwise(
      () => new Date(date.getFullYear(), date.getMonth(), date.getDate(), date.getHours()),
    );

export const getBucketOffset = (granularity: MetricGranularity, start: Date, index: number) =>
  match(granularity)
    .with(MetricGranularity.DAY, () =>
      new Date(start.getFullYear(), start.getMonth(), start.getDate() + index).getTime(),
    )
    .otherwise(() => start.getTime() + index * OBSERVABILITY_HOUR_MS);

export const createTimeframeSince = (
  timeframe: ObservabilityTimeframe,
  bucketStartMs: number,
): bigint => {
  const { durationMs, granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  return BigInt(
    bucketStartMs + OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP[granularity] - durationMs,
  );
};

export const createObservabilityTimeseriesInput = (
  timeframe: ObservabilityTimeframe,
  bucketStartMs: number,
  init: { metrics: Metric[]; groupBy: MetricDimension; filters?: MetricFilter[] },
): QueryTimeseriesRequest =>
  create(QueryTimeseriesRequestSchema, {
    ...init,
    sinceMs: createTimeframeSince(timeframe, bucketStartMs),
    granularity: OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
    tzOffsetMinutes: -new Date(bucketStartMs).getTimezoneOffset(),
  });
