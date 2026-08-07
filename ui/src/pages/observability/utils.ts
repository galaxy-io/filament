import { useEffect, useMemo, useState } from "react";

import { useSearch } from "@tanstack/react-router";

import { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

import { ObservabilityTimeframe } from "@/pages/observability/types";

import { METRICS_REFETCH_INTERVAL } from "@/api/queries/metrics";

const MINUTE_MS = 60 * 1000;
const DAY_MS = 24 * 60 * MINUTE_MS;

/** Unique per bucket, unlike the display label — safe to use as a chart category key. */
export const formatBucketKey = (bucketStartMs: bigint) => bucketStartMs.toString();

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

/**
 * A trailing window that keeps advancing: recomputed on a timer matching the
 * metrics refetch interval, and immediately whenever the toolbar's Refresh
 * button bumps `refreshedAt`. Without this, the window is frozen at whatever
 * moment the caller last mounted or the timeframe last changed.
 */
export const useSlidingTimeframeWindow = (
  timeframe: ObservabilityTimeframe,
): { sinceMs: bigint; untilMs: bigint } => {
  const { refreshedAt } = useSearch({ from: "/_main/observability" });
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => setTick((value) => value + 1), METRICS_REFETCH_INTERVAL);
    return () => clearInterval(interval);
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refreshedAt/tick intentionally force recomputation without being read in the body.
  return useMemo(() => createTimeframeWindow(timeframe), [timeframe, refreshedAt, tick]);
};

export const useBucketLabelFormatter = (
  timeframe: ObservabilityTimeframe,
): ((key: string) => string) =>
  useMemo(() => {
    const { formatBucketLabel } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
    return (key: string) => formatBucketLabel(BigInt(key));
  }, [timeframe]);
