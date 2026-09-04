import { Code, ConnectError } from "@connectrpc/connect";
import { QueryClient } from "@tanstack/react-query";

const DEFAULT_STALE_TIME = 30 * 1000;
const DEFAULT_REFETCH_ON_WINDOW_FOCUS = true;
const DEFAULT_RETRY_COUNT = 3;
const NON_RETRYABLE_CODES = new Set<Code>([
  Code.NotFound,
  Code.PermissionDenied,
  Code.Unauthenticated,
  Code.InvalidArgument,
]);

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: DEFAULT_STALE_TIME,
      refetchOnWindowFocus: DEFAULT_REFETCH_ON_WINDOW_FOCUS,
      retry: (failureCount, error) =>
        failureCount < DEFAULT_RETRY_COUNT &&
        !(error instanceof ConnectError && NON_RETRYABLE_CODES.has(error.code)),
    },
  },
});
