import { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

import {
  ObservabilityRefreshInterval,
  ObservabilityRunsView,
  ObservabilityTimeframe,
  type ObservabilityTimeframeQuery,
} from "@/pages/observability/types";

export const OBSERVABILITY_WIDGET_BASIS = "400px";
export const OBSERVABILITY_SCROLL_END_TOLERANCE_PX = 1;
export const OBSERVABILITY_WHEEL_LINE_PX = 16;

export const OBSERVABILITY_DEFAULT_TIMEFRAME = ObservabilityTimeframe.TWENTY_FOUR_HOURS;
export const OBSERVABILITY_DEFAULT_RUNS_VIEW = ObservabilityRunsView.PAST;
export const OBSERVABILITY_DEFAULT_REFRESH_INTERVAL = ObservabilityRefreshInterval.ONE_MINUTE;

const OBSERVABILITY_MINUTE_MS = 60 * 1000;
export const OBSERVABILITY_HOUR_MS = 60 * OBSERVABILITY_MINUTE_MS;
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

export const OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP: Record<ObservabilityTimeframe, string> = {
  [ObservabilityTimeframe.TWENTY_FOUR_HOURS]: "24 hours",
  [ObservabilityTimeframe.SEVEN_DAYS]: "7 days",
  [ObservabilityTimeframe.THIRTY_DAYS]: "30 days",
};

export const OBSERVABILITY_REFRESH_INTERVAL_TO_LABEL_MAP: Record<
  ObservabilityRefreshInterval,
  string
> = {
  [ObservabilityRefreshInterval.ONE_MINUTE]: "Every minute",
  [ObservabilityRefreshInterval.FIVE_MINUTES]: "Every 5 minutes",
  [ObservabilityRefreshInterval.FIFTEEN_MINUTES]: "Every 15 minutes",
  [ObservabilityRefreshInterval.THIRTY_MINUTES]: "Every 30 minutes",
  [ObservabilityRefreshInterval.ONE_HOUR]: "Every hour",
};

export const OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP: Record<
  ObservabilityRefreshInterval,
  number
> = {
  [ObservabilityRefreshInterval.ONE_MINUTE]: OBSERVABILITY_MINUTE_MS,
  [ObservabilityRefreshInterval.FIVE_MINUTES]: 5 * OBSERVABILITY_MINUTE_MS,
  [ObservabilityRefreshInterval.FIFTEEN_MINUTES]: 15 * OBSERVABILITY_MINUTE_MS,
  [ObservabilityRefreshInterval.THIRTY_MINUTES]: 30 * OBSERVABILITY_MINUTE_MS,
  [ObservabilityRefreshInterval.ONE_HOUR]: OBSERVABILITY_HOUR_MS,
};

export const OBSERVABILITY_REFRESH_INTERVAL_SELECT_WIDTH = 160;
