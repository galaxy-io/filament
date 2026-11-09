import type { QueryTimeseriesRequest } from "@/gen/metrics/v1/metrics_pb";

import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

export const useObservabilityTimeseriesQuery = (input: QueryTimeseriesRequest) => {
  const refetchInterval = useObservabilityRefetchInterval();
  return useQueryTimeseriesQuery({ input, options: { refetchInterval } });
};
