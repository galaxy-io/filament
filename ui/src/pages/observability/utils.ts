import { create } from "@bufbuild/protobuf";

import {
  type Metric,
  type MetricDimension,
  type MetricFilter,
  type QueryTimeseriesRequest,
  QueryTimeseriesRequestSchema,
  type TimeseriesPoint,
} from "@/gen/metrics/v1/metrics_pb";

import {
  OBSERVABILITY_MINUTE_MS,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";

export const formatBucketKey = (bucketStartMs: TimeseriesPoint["bucketStartMs"]) =>
  bucketStartMs.toString();

export const createTimeframeSince = (timeframe: ObservabilityTimeframe): bigint => {
  const { durationMs } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  const nowMs = Math.floor(Date.now() / OBSERVABILITY_MINUTE_MS) * OBSERVABILITY_MINUTE_MS;
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
