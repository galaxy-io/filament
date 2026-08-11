import { ChartPalette, type ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import { ObservabilityThroughputView, ObservabilityUsageView } from "@/pages/observability/types";

import { formatBytes, formatCount, formatSeconds } from "@/utils/format";

export const OBSERVABILITY_TIMESERIES_CHART_HEIGHT = 240;
export const OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH = 150;

export const METRIC_DIMENSION_PIVOT_OPTIONS: SelectInputOption[] = [
  {
    id: String(MetricDimension.PIPELINE_ID),
    label: "Pipeline",
    value: MetricDimension.PIPELINE_ID,
  },
  {
    id: String(MetricDimension.STATUS),
    label: "Status",
    value: MetricDimension.STATUS,
  },
];

export const OBSERVABILITY_TIMESERIES_PIVOT_PALETTE = [
  ChartPalette.PURPLE,
  ChartPalette.TEAL,
  ChartPalette.PINK,
  ChartPalette.ORANGE,
  ChartPalette.BLUE,
  ChartPalette.GREEN,
  ChartPalette.YELLOW,
];

export interface ObservabilityChartView {
  label: string;
  seriesLabel: string;
  metric: Metric;
  color: ChartPalette;
  valueFormatter: ChartValueFormatter;
}

export const OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP: Record<
  ObservabilityThroughputView,
  ObservabilityChartView
> = {
  [ObservabilityThroughputView.RECORDS]: {
    label: "Records",
    seriesLabel: "Records",
    metric: Metric.RUN_RECORDS,
    color: ChartPalette.PURPLE,
    valueFormatter: (value) => formatCount(BigInt(Math.round(value))),
  },
  [ObservabilityThroughputView.VOLUME]: {
    label: "Volume",
    seriesLabel: "Bytes",
    metric: Metric.RUN_BYTES,
    color: ChartPalette.TEAL,
    valueFormatter: (value) => formatBytes(BigInt(Math.round(value))),
  },
};

export const OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP: Record<
  ObservabilityUsageView,
  ObservabilityChartView
> = {
  [ObservabilityUsageView.CPU]: {
    label: "CPU",
    seriesLabel: "CPU",
    metric: Metric.RUN_CPU_USAGE,
    color: ChartPalette.ORANGE,
    valueFormatter: formatSeconds,
  },
  [ObservabilityUsageView.MEMORY]: {
    label: "Memory",
    seriesLabel: "Memory",
    metric: Metric.RUN_MEMORY_USAGE,
    color: ChartPalette.PINK,
    valueFormatter: (value) => formatBytes(BigInt(Math.round(value))),
  },
};
