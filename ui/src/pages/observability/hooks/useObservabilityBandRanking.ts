import { useMemo } from "react";

import type { QueryAggregateRequest } from "@/gen/metrics/v1/metrics_pb";

import { useObservabilityAggregateQuery } from "@/pages/observability/hooks/useObservabilityAggregateQuery";
import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";

export const useObservabilityBandRanking = (input: QueryAggregateRequest) => {
  const { pipelinesById } = useObservabilityPipelines();
  const { data, isLoading } = useObservabilityAggregateQuery(input);

  const ranking = useMemo(
    () =>
      (data?.rows ?? [])
        .flatMap((row) => {
          const pipeline = pipelinesById.get(row.key);
          const value = row.values[0] ?? 0;
          return pipeline && value > 0 ? [{ pipeline, value }] : [];
        })
        .sort((a, b) => b.value - a.value),
    [data, pipelinesById],
  );

  return { ranking, isLoading };
};
