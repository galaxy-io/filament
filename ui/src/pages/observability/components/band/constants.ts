import type { ChartSeriesStyles } from "@galaxy-io/dls/charts/types";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric } from "@/gen/metrics/v1/metrics_pb";

import { PIPELINE_RUN_EXECUTED_STATUSES } from "@/components/runs/constants";

import type { ObservabilityBandRunsMetric } from "@/pages/observability/components/band/types";

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

export const OBSERVABILITY_BAND_INFO_ICON_SIZE = 12;
export const OBSERVABILITY_BAND_COUNTS_HEIGHT = 20;
export const OBSERVABILITY_BAND_CHART_HEIGHT = 136;
export const OBSERVABILITY_BAND_VALUE_HEIGHT = 20;
export const OBSERVABILITY_BAND_TOP_COUNT = 3;
export const OBSERVABILITY_BAND_TOP_ROW_HEIGHT = 16;
export const OBSERVABILITY_BAND_THROUGHPUT_COLOR: RoleColor = "purple";
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

export const OBSERVABILITY_BAND_TOTAL_METRICS = [
  Metric.RUN_RECORDS,
  Metric.RUN_BYTES,
  Metric.RUN_CPU_USAGE,
  Metric.RUN_MEMORY_USAGE,
];
