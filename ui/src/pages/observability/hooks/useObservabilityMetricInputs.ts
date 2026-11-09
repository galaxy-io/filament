import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension, QueryAggregateRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_TOTAL_METRICS } from "@/pages/observability/components/band/constants";
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import {
  createObservabilityStatusFilter,
  createObservabilityTimeseriesInput,
  createTimeframeSince,
  getBucketLabelFormatter,
} from "@/pages/observability/utils";

import { useObservabilitySearch } from "@/module/hooks";

export const useObservabilityMetricInputs = () => {
  const { timeframe } = useObservabilitySearch();
  const bucketStartMs = useObservabilityBucketStart(timeframe);

  return useMemo(() => {
    const sinceMs = createTimeframeSince(timeframe, bucketStartMs);
    return {
      bucketStartMs,
      labelFormatter: getBucketLabelFormatter(
        OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
      ),
      totalsInput: create(QueryAggregateRequestSchema, {
        metrics: OBSERVABILITY_BAND_TOTAL_METRICS,
        sinceMs,
      }),
      statusCountsInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_COUNT],
        sinceMs,
        groupBy: MetricDimension.STATUS,
      }),
      createRankingInput: (metric: Metric, statuses?: RunStatus[]) =>
        create(QueryAggregateRequestSchema, {
          metrics: [metric],
          sinceMs,
          groupBy: MetricDimension.PIPELINE_ID,
          filters: statuses ? [createObservabilityStatusFilter(statuses)] : [],
        }),
      createSeriesInput: (metric: Metric, groupBy: MetricDimension, statuses?: RunStatus[]) =>
        createObservabilityTimeseriesInput(timeframe, bucketStartMs, {
          metrics: [metric],
          groupBy,
          filters: statuses ? [createObservabilityStatusFilter(statuses)] : [],
        }),
    };
  }, [timeframe, bucketStartMs]);
};
