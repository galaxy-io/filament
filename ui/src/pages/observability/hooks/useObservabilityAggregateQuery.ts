import type { QueryAggregateRequest } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP } from "@/pages/observability/constants";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryAggregateQuery } from "@/api/queries/metrics";

export const useObservabilityAggregateQuery = (input: QueryAggregateRequest) => {
  const { refreshInterval } = useObservabilitySearch();
  return useQueryAggregateQuery({
    input,
    options: { refetchInterval: OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP[refreshInterval] },
  });
};
