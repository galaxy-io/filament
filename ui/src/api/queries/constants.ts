export const PROBE_QUERY_OPTIONS = {
  retry: false,
  networkMode: "always",
  staleTime: Number.POSITIVE_INFINITY,
  refetchOnWindowFocus: false,
} as const;

export const ACTIVE_RUNS_REFETCH_INTERVAL = 3 * 1000;

export const ACTIVE_PIPELINE_RUNS_PAGE_SIZE = 8;
