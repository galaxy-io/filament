import path from "node:path";

import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";

import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { galaxyDls } from "@galaxy-io/dls/vite";

import { HOST_RUNTIME_CONFIG_ELEMENT_ID } from "./src/host/constants";
import { GalaxyProduct } from "./src/module/types";

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
const devRuntimeConfig = (): Plugin => ({
  name: "filament-dev-runtime-config",
  apply: "serve",
  transformIndexHtml: () => [
    {
      tag: "script",
      attrs: { type: "application/json", id: HOST_RUNTIME_CONFIG_ELEMENT_ID },
      children: JSON.stringify({
        products: Object.fromEntries(
          Object.values(GalaxyProduct).flatMap((product) => {
            const url = process.env[`GALAXY_${product.toUpperCase()}_URL`];
            return url ? [[product, url]] : [];
          }),
        ),
      }),
      injectTo: "head-prepend",
    },
  ],
});

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
    devRuntimeConfig(),
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
