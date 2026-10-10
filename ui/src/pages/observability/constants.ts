import { create } from "@bufbuild/protobuf";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { ListPipelinesRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";
import { MetricGranularity } from "@/gen/metrics/v1/metrics_pb";

import {
  ObservabilityRunsView,
  ObservabilityTimeframe,
  type ObservabilityTimeframeQuery,
} from "@/pages/observability/types";

export const OBSERVABILITY_WIDGET_BASIS = "400px";

export const OBSERVABILITY_DEFAULT_TIMEFRAME = ObservabilityTimeframe.TWENTY_FOUR_HOURS;
export const OBSERVABILITY_DEFAULT_RUNS_VIEW = ObservabilityRunsView.PAST;

export const OBSERVABILITY_MINUTE_MS = 60 * 1000;
const OBSERVABILITY_HOUR_MS = 60 * OBSERVABILITY_MINUTE_MS;
const OBSERVABILITY_DAY_MS = 24 * OBSERVABILITY_HOUR_MS;

export const OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP: Record<MetricGranularity, bigint> = {
  [MetricGranularity.UNSPECIFIED]: 0n,
  [MetricGranularity.HOUR]: BigInt(OBSERVABILITY_HOUR_MS),
  [MetricGranularity.DAY]: BigInt(OBSERVABILITY_DAY_MS),
};

const startOfHour = (date: Date) =>
  new Date(date.getFullYear(), date.getMonth(), date.getDate(), date.getHours());

const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate());

export const OBSERVABILITY_GRANULARITY_TO_BUCKET_START_MAP: Record<
  MetricGranularity,
  (date: Date) => Date
> = {
  [MetricGranularity.UNSPECIFIED]: startOfHour,
  [MetricGranularity.HOUR]: startOfHour,
  [MetricGranularity.DAY]: startOfDay,
};

export const OBSERVABILITY_GRANULARITY_TO_BUCKET_OFFSET_MAP: Record<
  MetricGranularity,
  (start: Date, index: number) => number
> = {
  [MetricGranularity.UNSPECIFIED]: (start, index) =>
    start.getTime() + index * OBSERVABILITY_HOUR_MS,
  [MetricGranularity.HOUR]: (start, index) => start.getTime() + index * OBSERVABILITY_HOUR_MS,
  [MetricGranularity.DAY]: (start, index) =>
    new Date(start.getFullYear(), start.getMonth(), start.getDate() + index).getTime(),
};

export const OBSERVABILITY_CONNECTIONS_INPUT = create(ListConnectionsRequestSchema, {
  includeDeleted: true,
});

export const OBSERVABILITY_PIPELINES_INPUT = create(ListPipelinesRequestSchema, {
  includeDeleted: true,
});

const formatHourBucketLabel = (bucketKey: string) =>
  `${new Date(Number(bucketKey)).getHours().toString().padStart(2, "0")}:00`;

const formatDayBucketLabel = (bucketKey: string) => {
  const date = new Date(Number(bucketKey));
  return `${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")}`;
};

export const OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP: Record<
  ObservabilityTimeframe,
  ObservabilityTimeframeQuery
> = {
  [ObservabilityTimeframe.TWENTY_FOUR_HOURS]: {
    durationMs: OBSERVABILITY_DAY_MS,
    granularity: MetricGranularity.HOUR,
    formatBucketLabel: formatHourBucketLabel,
  },
  [ObservabilityTimeframe.SEVEN_DAYS]: {
    durationMs: 7 * OBSERVABILITY_DAY_MS,
    granularity: MetricGranularity.DAY,
    formatBucketLabel: formatDayBucketLabel,
  },
  [ObservabilityTimeframe.THIRTY_DAYS]: {
    durationMs: 30 * OBSERVABILITY_DAY_MS,
    granularity: MetricGranularity.DAY,
    formatBucketLabel: formatDayBucketLabel,
  },
};
