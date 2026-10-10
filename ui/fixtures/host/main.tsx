import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { Code, ConnectError } from "@connectrpc/connect";
import { TransportProvider } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";

import GalaxyProvider from "@galaxy-io/dls/theme/GalaxyProvider";

import { router } from "./router";

import "@galaxy-io/dls/styles.css";
import "@galaxy-io/dls/tokens.css";
import "@galaxy-io/dls/fonts.css";
import "@galaxy-io/filament/styles.css";

const DECOY_BASE_URL = "/elsewhere";
const STALE_TIME = 30_000;
const MAX_RETRIES = 3;
const NON_RETRYABLE_CODES = new Set<Code>([
  Code.NotFound,
  Code.PermissionDenied,
  Code.Unauthenticated,
  Code.InvalidArgument,
  Code.Unimplemented,
]);

const decoyTransport = createConnectTransport({ baseUrl: DECOY_BASE_URL });

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: STALE_TIME,
      retry: (failureCount, error) =>
        failureCount < MAX_RETRIES && !NON_RETRYABLE_CODES.has(ConnectError.from(error).code),
    },
  },
});

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <GalaxyProvider>
      <QueryClientProvider client={queryClient}>
        <TransportProvider transport={decoyTransport}>
          <RouterProvider router={router} />
        </TransportProvider>
      </QueryClientProvider>
    </GalaxyProvider>
  </StrictMode>,
);
