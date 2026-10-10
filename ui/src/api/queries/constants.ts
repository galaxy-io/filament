export const PROBE_QUERY_OPTIONS = {
  retry: false,
  networkMode: "always",
  staleTime: Number.POSITIVE_INFINITY,
  refetchOnWindowFocus: false,
} as const;

export const ACTIVE_RUNS_REFETCH_INTERVAL = 3 * 1000;
