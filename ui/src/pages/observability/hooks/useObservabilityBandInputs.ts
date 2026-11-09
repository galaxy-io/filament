import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension, QueryAggregateRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_TOTAL_METRICS } from "@/pages/observability/components/band/constants";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import { createObservabilityStatusFilter, createTimeframeSince } from "@/pages/observability/utils";

import { useObservabilitySearch } from "@/module/hooks";

export const useObservabilityBandInputs = () => {
  const { timeframe } = useObservabilitySearch();
  const bucketStartMs = useObservabilityBucketStart(timeframe);

  return useMemo(() => {
    const sinceMs = createTimeframeSince(timeframe, bucketStartMs);
    return {
      totalsInput: create(QueryAggregateRequestSchema, {
        metrics: OBSERVABILITY_BAND_TOTAL_METRICS,
        sinceMs,
      }),
      statusCountsInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_COUNT],
        sinceMs,
        groupBy: MetricDimension.STATUS,
      }),
      failuresByPipelineInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_COUNT],
        sinceMs,
        groupBy: MetricDimension.PIPELINE_ID,
        filters: [createObservabilityStatusFilter([RunStatus.FAILED])],
      }),
      recordsByPipelineInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_RECORDS],
        sinceMs,
        groupBy: MetricDimension.PIPELINE_ID,
      }),
      cpuByPipelineInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_CPU_USAGE],
        sinceMs,
        groupBy: MetricDimension.PIPELINE_ID,
      }),
    };
  }, [timeframe, bucketStartMs]);
};
