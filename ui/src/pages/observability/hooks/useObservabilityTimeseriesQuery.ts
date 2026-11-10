import type { QueryTimeseriesRequest } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP } from "@/pages/observability/constants";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

export const useObservabilityTimeseriesQuery = (input: QueryTimeseriesRequest) => {
  const { refreshInterval } = useObservabilitySearch();
  return useQueryTimeseriesQuery({
    input,
    options: { refetchInterval: OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP[refreshInterval] },
  });
};
