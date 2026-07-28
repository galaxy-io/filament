import { QueryClient } from "@tanstack/react-query";

const DEFAULT_STALE_TIME = 30 * 1000;
const DEFAULT_REFETCH_ON_WINDOW_FOCUS = true;

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: DEFAULT_STALE_TIME,
      refetchOnWindowFocus: DEFAULT_REFETCH_ON_WINDOW_FOCUS,
    },
  },
});
