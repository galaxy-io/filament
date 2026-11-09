import { useMemo } from "react";

import type { QueryAggregateRequest } from "@/gen/metrics/v1/metrics_pb";

import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";
import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";

import { useQueryAggregateQuery } from "@/api/queries/metrics";

export const useObservabilityBandRanking = (input: QueryAggregateRequest) => {
  const refetchInterval = useObservabilityRefetchInterval();
  const { pipelinesById } = useObservabilityPipelines();
  const { data, isLoading } = useQueryAggregateQuery({
    input,
    options: { refetchInterval },
  });

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
