import "./utils/serialization";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import App from "@/App";

import "@galaxy-io/dls/styles.css";
import "@/style.css";

import { GalaxyThemeProvider } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

import TransportQueryClientProvider from "@/api/TransportQueryClientProvider";

import AppAuthProvider from "@/auth/AppAuthProvider";

const GALAXY_THEME_VALUES = new Set<string>(Object.values(GalaxyTheme));

const themeStorage = {
  get: (key: string): GalaxyTheme | null => {
    const theme = window.localStorage.getItem(key);
    return theme && GALAXY_THEME_VALUES.has(theme) ? (theme as GalaxyTheme) : null;
  },
  set: (key: string, theme: GalaxyTheme) => {
    window.localStorage.setItem(key, theme);
  },
};

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}
const root = createRoot(rootElement);
root.render(
  <StrictMode>
    <GalaxyThemeProvider initialTheme={GalaxyTheme.SYSTEM} storage={themeStorage}>
      <TransportQueryClientProvider>
        <AppAuthProvider>
          <App />
        </AppAuthProvider>
      </TransportQueryClientProvider>
    </GalaxyThemeProvider>
  </StrictMode>,
);
