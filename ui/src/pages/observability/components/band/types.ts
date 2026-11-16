import type { ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import type { Metric, TimeseriesPoint } from "@/gen/metrics/v1/metrics_pb";

export type ObservabilityBandRunsMetric = "runs";

export interface ObservabilityBandSeriesEntry {
  key: string;
  label: string;
  color: RoleColor;
  points: TimeseriesPoint[];
}

export interface ObservabilityBandViewConfig {
  label: string;
  info: string;
  unit: string;
  topTitle: string;
  emptyText: string;
  metric: Metric;
  color: RoleColor;
  format: (value: number) => string;
  axisFormat: ChartValueFormatter;
}

export interface ObservabilityBandListItem {
  pipeline: Pipeline;
  value: string;
  tooltip?: string;
}
