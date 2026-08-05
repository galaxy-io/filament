import { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

import { ObservabilityTimeframe } from "@/pages/observability/types";

const MINUTE_MS = 60 * 1000;
const DAY_MS = 24 * 60 * MINUTE_MS;

const formatHourBucketLabel = (bucketStartMs: bigint) =>
  `${new Date(Number(bucketStartMs)).getHours().toString().padStart(2, "0")}:00`;

const formatDayBucketLabel = (bucketStartMs: bigint) => {
  const date = new Date(Number(bucketStartMs));
  return `${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")}`;
};

interface ObservabilityTimeframeQuery {
  durationMs: number;
  granularity: MetricGranularity;
  formatBucketLabel: (bucketStartMs: bigint) => string;
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

export const createTimeframeWindow = (
  timeframe: ObservabilityTimeframe,
): { sinceMs: bigint; untilMs: bigint } => {
  const { durationMs } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
  const untilMs = Math.floor(Date.now() / MINUTE_MS) * MINUTE_MS;
  return { sinceMs: BigInt(untilMs - durationMs), untilMs: BigInt(untilMs) };
};
