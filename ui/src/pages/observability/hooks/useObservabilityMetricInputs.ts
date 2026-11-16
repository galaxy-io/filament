import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import {
  type Metric,
  MetricDimension,
  QueryAggregateRequestSchema,
} from "@/gen/metrics/v1/metrics_pb";

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
      sinceMs,
      labelFormatter: getBucketLabelFormatter(
        OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
      ),
      createAggregateInput: (
        metric: Metric,
        groupBy = MetricDimension.UNSPECIFIED,
        statuses?: RunStatus[],
      ) =>
        create(QueryAggregateRequestSchema, {
          metrics: [metric],
          sinceMs,
          groupBy,
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
