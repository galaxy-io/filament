import "./utils/serialization";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import App from "@/App";

import "@galaxy-io/dls/styles.css";
import "@/style.css";

import { GalaxyThemeProvider } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

import TransportQueryClientProvider from "@/api/TransportQueryClientProvider";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}
const root = createRoot(rootElement);
root.render(
  <StrictMode>
    <TransportQueryClientProvider>
      <GalaxyThemeProvider initialTheme={GalaxyTheme.SYSTEM}>
        <App />
      </GalaxyThemeProvider>
    </TransportQueryClientProvider>
  </StrictMode>,
);
