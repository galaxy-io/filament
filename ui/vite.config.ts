import path from "node:path";

import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";

import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { galaxyDls } from "@galaxy-io/dls/vite";

export const CLASS_PREFIX = "filament";
const API_PROXY_TARGET = process.env.API_PROXY_TARGET ?? "http://localhost:8080";
const API_SERVICE_PATHS = [
  "/ingestion.v1.IngestionService",
  "/metrics.v1.MetricsService",
  "/auth.v1.AuthService",
];

export const API_PROXY = Object.fromEntries(
  API_SERVICE_PATHS.map((servicePath) => [
    servicePath,
    { target: API_PROXY_TARGET, changeOrigin: true },
  ]),
);
const VERIFY_MODE = "verify";
const VERIFY_CACHE_DIR = "node_modules/.vite-verify";

export default defineConfig(({ mode }) => ({
  cacheDir: mode === VERIFY_MODE ? VERIFY_CACHE_DIR : undefined,
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      routesDirectory: "./src/host/routes",
      generatedRouteTree: "./src/host/routeTree.gen.ts",
    }),
    viteReact(),
    galaxyDls({ prefix: CLASS_PREFIX, theme: GalaxyTheme.SYSTEM }),
  ],
  server: {
    port: 5173,
    strictPort: true,
    proxy: API_PROXY,
  },
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  build: {
    outDir: "build",
    target: "esnext",
    rolldownOptions: {
      onwarn(warning, warn) {
        if (
          warning.code === "PURE_COMMENT_POSITION" ||
          warning.message?.includes("contains an annotation that Rollup cannot interpret")
        ) {
          return;
        }
        warn(warning);
      },
    },
  },
}));
