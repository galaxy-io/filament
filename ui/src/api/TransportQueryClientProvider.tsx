import type { PropsWithChildren } from "react";

import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClientProvider } from "@tanstack/react-query";

import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const TransportQueryClientProvider = ({ children }: PropsWithChildren) => {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  );
};

export default TransportQueryClientProvider;
