import { QueryClient } from "@tanstack/react-query";

const DEFAULT_STALE_TIME = 30 * 1000;

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: DEFAULT_STALE_TIME,
      refetchOnWindowFocus: true,
    },
  },
});
