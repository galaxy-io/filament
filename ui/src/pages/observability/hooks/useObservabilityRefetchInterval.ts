import { OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP } from "@/pages/observability/constants";

import { useObservabilitySearch } from "@/module/hooks";

export const useObservabilityRefetchInterval = () => {
  const { refreshInterval } = useObservabilitySearch();
  return OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP[refreshInterval];
};
