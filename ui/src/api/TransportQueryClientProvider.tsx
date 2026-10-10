import type { FC, PropsWithChildren } from "react";

import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClientProvider } from "@tanstack/react-query";

import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const TransportQueryClientProvider: FC<PropsWithChildren> = ({ children }) => {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  );
};

export default TransportQueryClientProvider;
