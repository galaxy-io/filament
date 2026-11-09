import type { QueryAggregateRequest } from "@/gen/metrics/v1/metrics_pb";

import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";

import { useQueryAggregateQuery } from "@/api/queries/metrics";

export const useObservabilityAggregateQuery = (input: QueryAggregateRequest) => {
  const refetchInterval = useObservabilityRefetchInterval();
  return useQueryAggregateQuery({ input, options: { refetchInterval } });
};
