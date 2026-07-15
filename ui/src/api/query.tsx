import { type PropsWithChildren, useMemo } from "react";

import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { createApiTransport } from "@/api/transport";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000,
      refetchOnWindowFocus: false,
    },
  },
});

const ApiQueryClientProvider = ({ children }: PropsWithChildren) => {
  const transport = useMemo(() => createApiTransport(), []);

  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  );
};

export default ApiQueryClientProvider;
