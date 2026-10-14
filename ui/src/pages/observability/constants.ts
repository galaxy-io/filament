import { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

import {
  ObservabilityRunsView,
  ObservabilityTimeframe,
  type ObservabilityTimeframeQuery,
} from "@/pages/observability/types";

export const OBSERVABILITY_WIDGET_BASIS = "400px";

export const OBSERVABILITY_DEFAULT_TIMEFRAME = ObservabilityTimeframe.TWENTY_FOUR_HOURS;
export const OBSERVABILITY_DEFAULT_RUNS_VIEW = ObservabilityRunsView.PAST;

export const OBSERVABILITY_HOUR_MS = 60 * 60 * 1000;
const OBSERVABILITY_DAY_MS = 24 * OBSERVABILITY_HOUR_MS;
export const OBSERVABILITY_DAY_BUCKET_MIDPOINT_MS = OBSERVABILITY_DAY_MS / 2;

export const OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP: Record<MetricGranularity, number> = {
  [MetricGranularity.UNSPECIFIED]: OBSERVABILITY_HOUR_MS,
  [MetricGranularity.HOUR]: OBSERVABILITY_HOUR_MS,
  [MetricGranularity.DAY]: OBSERVABILITY_DAY_MS,
};

export const OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP: Record<
  ObservabilityTimeframe,
  ObservabilityTimeframeQuery
> = {
  [ObservabilityTimeframe.TWENTY_FOUR_HOURS]: {
    durationMs: OBSERVABILITY_DAY_MS,
    granularity: MetricGranularity.HOUR,
  },
  [ObservabilityTimeframe.SEVEN_DAYS]: {
    durationMs: 7 * OBSERVABILITY_DAY_MS,
    granularity: MetricGranularity.DAY,
  },
  [ObservabilityTimeframe.THIRTY_DAYS]: {
    durationMs: 30 * OBSERVABILITY_DAY_MS,
    granularity: MetricGranularity.DAY,
  },
};
