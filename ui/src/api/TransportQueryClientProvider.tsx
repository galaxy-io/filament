import type { PropsWithChildren } from "react";

import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { transport } from "@/api/transport";

const DEFAULT_STALE_TIME = 30 * 1000;

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: DEFAULT_STALE_TIME,
      refetchOnWindowFocus: true,
    },
  },
});

const TransportQueryClientProvider = ({ children }: PropsWithChildren) => {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  );
};

export default TransportQueryClientProvider;
