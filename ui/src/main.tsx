import "./utils/serialization";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import App from "@/App";

import "@galaxy-io/dls/styles.css";
import "@/style.css";

import TransportQueryClientProvider from "@/api/TransportQueryClientProvider";

import ThemeProvider from "@/providers/theme/ThemeProvider";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}
const root = createRoot(rootElement);
root.render(
  <StrictMode>
    <ThemeProvider>
      <TransportQueryClientProvider>
        <App />
      </TransportQueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
);
