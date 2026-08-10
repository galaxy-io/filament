import { ChartPalette, type ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import { ObservabilityMetricView } from "@/pages/observability/types";

import { formatBytes, formatCount } from "@/utils/format";

interface ObservabilityMetricViewConfig {
  seriesLabel: string;
  metric: Metric;
  color: ChartPalette;
  valueFormatter: ChartValueFormatter;
}

export const OBSERVABILITY_METRIC_VIEW_TO_CONFIG_MAP: Record<
  ObservabilityMetricView,
  ObservabilityMetricViewConfig
> = {
  [ObservabilityMetricView.RECORDS]: {
    seriesLabel: "Records",
    metric: Metric.RUN_RECORDS,
    color: ChartPalette.PURPLE,
    valueFormatter: (value) => formatCount(BigInt(Math.round(value))),
  },
  [ObservabilityMetricView.VOLUME]: {
    seriesLabel: "Bytes",
    metric: Metric.RUN_BYTES,
    color: ChartPalette.TEAL,
    valueFormatter: (value) => formatBytes(BigInt(Math.round(value))),
  },
};

export const OBSERVABILITY_METRIC_VIEW_TO_LABEL_MAP: Record<ObservabilityMetricView, string> = {
  [ObservabilityMetricView.RECORDS]: "Records",
  [ObservabilityMetricView.VOLUME]: "Volume",
};

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
