import type { ChartSeriesStyles } from "@galaxy-io/dls/charts/types";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";
import {
  EMPTY_VALUE,
  formatBytes,
  formatDuration,
  formatNumber,
} from "@galaxy-io/dls/utils/format";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import { PIPELINE_RUN_EXECUTED_STATUSES } from "@/components/runs/constants";

import type {
  ObservabilityBandRunsMetric,
  ObservabilityBandViewConfig,
} from "@/pages/observability/components/band/types";
import {
  type ObservabilityBandViewKey,
  type ObservabilityBandViews,
  ObservabilityThroughputView,
  ObservabilityUsageView,
} from "@/pages/observability/types";

export const OBSERVABILITY_BAND_FEATURED_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

export const OBSERVABILITY_BAND_OTHER_STATUSES = PIPELINE_RUN_EXECUTED_STATUSES.filter(
  (status) => !OBSERVABILITY_BAND_FEATURED_STATUSES.includes(status),
);

export const OBSERVABILITY_BAND_RUNS_SERIES: ChartSeriesStyles<ObservabilityBandRunsMetric> = {
  runs: { label: "Runs" },
};

export const OBSERVABILITY_BAND_WRAP_MAX_WIDTH = 1120;
export const OBSERVABILITY_BAND_INFO_ICON_SIZE = 12;
export const OBSERVABILITY_BAND_HEADER_HEIGHT = 24;
export const OBSERVABILITY_BAND_COUNTS_HEIGHT = 20;
export const OBSERVABILITY_BAND_CHART_HEIGHT = 136;
export const OBSERVABILITY_BAND_VALUE_HEIGHT = 20;
export const OBSERVABILITY_BAND_TOP_COUNT = 3;
export const OBSERVABILITY_BAND_TOP_ROW_HEIGHT = 16;
export const OBSERVABILITY_BAND_PIVOT_SELECT_WIDTH = 110;

export const OBSERVABILITY_BAND_PIVOT_OPTIONS: SelectOption[] = [
  { id: String(MetricDimension.PIPELINE_ID), label: "Pipeline" },
  { id: String(MetricDimension.STATUS), label: "Status" },
];
export const OBSERVABILITY_BAND_VIEW_KEY_TO_CONFIG_MAP: {
  [TKey in ObservabilityBandViewKey]: Record<
    ObservabilityBandViews[TKey],
    ObservabilityBandViewConfig
  >;
} = {
  throughput: {
    [ObservabilityThroughputView.RECORDS]: {
      label: "Records",
      info: "Records written by runs",
      unit: "records",
      topTitle: "Most records",
      emptyText: "No records written",
      metric: Metric.RUN_RECORDS,
      color: "purple",
      format: (value) => formatNumber(value, { precision: 0 }),
      axisFormat: (value) => formatNumber(value, { precision: 0 }),
    },
    [ObservabilityThroughputView.VOLUME]: {
      label: "Volume",
      info: "Bytes written by runs",
      unit: "written",
      topTitle: "Most volume",
      emptyText: "No bytes written",
      metric: Metric.RUN_BYTES,
      color: "teal",
      format: (value) => formatBytes(value),
      axisFormat: (value) => formatBytes(value),
    },
  },
  usage: {
    [ObservabilityUsageView.CPU]: {
      label: "CPU",
      info: "Average CPU time of one run, across runs",
      unit: "per run",
      topTitle: "Heaviest runs",
      emptyText: "No usage reported",
      metric: Metric.RUN_CPU_USAGE,
      color: "orange",
      format: (value) => (value > 0 ? formatDuration(value * 1_000) : EMPTY_VALUE),
      axisFormat: (value) => formatDuration(value * 1_000),
    },
    [ObservabilityUsageView.MEMORY]: {
      label: "Memory",
      info: "Average peak memory of one run, across runs",
      unit: "peak per run",
      topTitle: "Heaviest runs",
      emptyText: "No usage reported",
      metric: Metric.RUN_MEMORY_USAGE,
      color: "pink",
      format: (value) => (value > 0 ? formatBytes(value) : EMPTY_VALUE),
      axisFormat: (value) => formatBytes(value),
    },
  },
};

export const OBSERVABILITY_BAND_PIPELINE_COLORS: RoleColor[] = [
  "orange",
  "teal",
  "pink",
  "purple",
  "blue",
  "green",
  "yellow",
];
export const OBSERVABILITY_BAND_RUNS_MIN_SEGMENT_LENGTH = 4;
export const OBSERVABILITY_BAND_VALUE_SKELETON_WIDTH = 96;
