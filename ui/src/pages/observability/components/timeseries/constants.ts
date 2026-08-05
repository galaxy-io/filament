import { ChartPalette } from "@galaxy-io/dls/charts/types";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

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
