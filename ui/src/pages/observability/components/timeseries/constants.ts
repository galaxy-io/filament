import { ChartPalette, type ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import { formatBytes, formatDuration, formatNumber } from "@galaxy-io/dls/utils/format";

import { Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import { ObservabilityThroughputView, ObservabilityUsageView } from "@/pages/observability/types";

export const OBSERVABILITY_TIMESERIES_CHART_HEIGHT = 240;
export const OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH = 150;

export const OBSERVABILITY_TIMESERIES_PIVOT_OPTIONS: SelectOption[] = [
  {
    id: String(MetricDimension.PIPELINE_ID),
    label: "Pipeline",
  },
  {
    id: String(MetricDimension.STATUS),
    label: "Status",
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

export const OBSERVABILITY_THROUGHPUT_VIEWS = [
  ObservabilityThroughputView.RECORDS,
  ObservabilityThroughputView.VOLUME,
];

export const OBSERVABILITY_USAGE_VIEWS = [
  ObservabilityUsageView.CPU,
  ObservabilityUsageView.MEMORY,
];

export const OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP: Record<
  ObservabilityThroughputView,
  ObservabilityChartView
> = {
  [ObservabilityThroughputView.RECORDS]: {
    label: "Records",
    seriesLabel: "Records",
    metric: Metric.RUN_RECORDS,
    color: ChartPalette.PURPLE,
    valueFormatter: (value) => formatNumber(value, { precision: 0 }),
  },
  [ObservabilityThroughputView.VOLUME]: {
    label: "Volume",
    seriesLabel: "Bytes",
    metric: Metric.RUN_BYTES,
    color: ChartPalette.TEAL,
    valueFormatter: (value) => formatBytes(value),
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
    valueFormatter: (value) => formatDuration(value * 1_000),
  },
  [ObservabilityUsageView.MEMORY]: {
    label: "Memory",
    seriesLabel: "Memory",
    metric: Metric.RUN_MEMORY_USAGE,
    color: ChartPalette.PINK,
    valueFormatter: (value) => formatBytes(value),
  },
};
