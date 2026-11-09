import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import type { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

export enum ObservabilityTimeframe {
  TWENTY_FOUR_HOURS = "24H",
  SEVEN_DAYS = "7D",
  THIRTY_DAYS = "30D",
}

export enum ObservabilityRefreshInterval {
  ONE_MINUTE = "1M",
  FIVE_MINUTES = "5M",
  FIFTEEN_MINUTES = "15M",
  THIRTY_MINUTES = "30M",
  ONE_HOUR = "1H",
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

export interface ObservabilityBandViews {
  throughput: ObservabilityThroughputView;
  usage: ObservabilityUsageView;
}

export type ObservabilityBandViewKey = keyof ObservabilityBandViews;

export type ObservabilityBandPivotKey = "throughputPivot" | "usagePivot";

export interface ObservabilityTimeframeQuery {
  durationMs: number;
  granularity: MetricGranularity;
}

export enum ObservabilityPipelineHealth {
  RUNNING = "RUNNING",
  FAILING = "FAILING",
  HEALTHY = "HEALTHY",
}

export interface ObservabilityPipelineEntry {
  pipeline: Pipeline;
  health: ObservabilityPipelineHealth;
}
