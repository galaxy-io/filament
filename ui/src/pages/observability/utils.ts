import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import {
  type Metric,
  type MetricDimension,
  type MetricFilter,
  MetricGranularity,
  type QueryTimeseriesRequest,
  QueryTimeseriesRequestSchema,
  type TimeseriesPoint,
} from "@/gen/metrics/v1/metrics_pb";

import { ObservabilityTimeframe } from "@/pages/observability/types";

const MINUTE_MS = 60 * 1000;
const DAY_MS = 24 * 60 * MINUTE_MS;

export const formatBucketKey = (bucketStartMs: TimeseriesPoint["bucketStartMs"]) =>
  bucketStartMs.toString();

const formatHourBucketLabel = (bucketStartMs: TimeseriesPoint["bucketStartMs"]) =>
  `${new Date(Number(bucketStartMs)).getHours().toString().padStart(2, "0")}:00`;

const formatDayBucketLabel = (bucketStartMs: TimeseriesPoint["bucketStartMs"]) => {
  const date = new Date(Number(bucketStartMs));
  return `${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")}`;
};

interface ObservabilityTimeframeQuery {
  durationMs: number;
  granularity: MetricGranularity;
  formatBucketLabel: (bucketStartMs: TimeseriesPoint["bucketStartMs"]) => string;
}

export const OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP: Record<
  ObservabilityTimeframe,
  ObservabilityTimeframeQuery
> = {
  [ObservabilityTimeframe.TWENTY_FOUR_HOURS]: {
    durationMs: DAY_MS,
    granularity: MetricGranularity.HOUR,
    formatBucketLabel: formatHourBucketLabel,
  },
  [ObservabilityTimeframe.SEVEN_DAYS]: {
    durationMs: 7 * DAY_MS,
    granularity: MetricGranularity.DAY,
    formatBucketLabel: formatDayBucketLabel,
  },
  [ObservabilityTimeframe.THIRTY_DAYS]: {
    durationMs: 30 * DAY_MS,
    granularity: MetricGranularity.DAY,
    formatBucketLabel: formatDayBucketLabel,
  },
};

export const OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP: Record<MetricGranularity, bigint> = {
  [MetricGranularity.UNSPECIFIED]: 0n,
  [MetricGranularity.HOUR]: BigInt(60 * MINUTE_MS),
  [MetricGranularity.DAY]: BigInt(DAY_MS),
};

export const createTimeframeSince = (timeframe: ObservabilityTimeframe): bigint => {
  const { durationMs } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  const nowMs = Math.floor(Date.now() / MINUTE_MS) * MINUTE_MS;
  return BigInt(nowMs - durationMs);
};

export const createObservabilityTimeseriesInput = (
  timeframe: ObservabilityTimeframe,
  init: { metrics: Metric[]; groupBy: MetricDimension; filters?: MetricFilter[] },
): QueryTimeseriesRequest =>
  create(QueryTimeseriesRequestSchema, {
    ...init,
    sinceMs: createTimeframeSince(timeframe),
    granularity: OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
    tzOffsetMinutes: -new Date().getTimezoneOffset(),
  });

export const useBucketLabelFormatter = (
  timeframe: ObservabilityTimeframe,
): ((key: string) => string) =>
  useMemo(() => {
    const { formatBucketLabel } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
    return (key: string) => formatBucketLabel(BigInt(key));
  }, [timeframe]);
