import type { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

export enum ObservabilityTimeframe {
  TWENTY_FOUR_HOURS = "24H",
  SEVEN_DAYS = "7D",
  THIRTY_DAYS = "30D",
}

export enum ObservabilityRunsView {
  PAST = "PAST",
  UPCOMING = "UPCOMING",
}

export enum ObservabilityThroughputView {
  RECORDS = "RECORDS",
  VOLUME = "VOLUME",
}

export enum ObservabilityUsageView {
  CPU = "CPU",
  MEMORY = "MEMORY",
}

export interface ObservabilityTimeframeQuery {
  durationMs: number;
  granularity: MetricGranularity;
  formatBucketLabel: (bucketKey: string) => string;
}
