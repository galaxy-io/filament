import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import App from "@/App";

import "@galaxy-io/dls/styles.css";
import "@/style.css";

import ThemeProvider from "@/providers/theme/ThemeProvider";

import ApiQueryClientProvider from "@/api/query";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}
const root = createRoot(rootElement);
root.render(
  <StrictMode>
    <ApiQueryClientProvider>
      <ThemeProvider>
        <App />
      </ThemeProvider>
    </ApiQueryClientProvider>
  </StrictMode>,
);
