import path from "node:path";

import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import wyw from "@wyw-in-js/vite";
import { defineConfig } from "vite";

const API_PROXY_TARGET = process.env.API_PROXY_TARGET ?? "http://localhost:8080";
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
    wyw({
      include: ["**/*.{ts,tsx}"],
      exclude: ["**/node_modules/**", "**/dist/**", "**/*.d.ts", "**/src/gen/**"],
      babelOptions: {
        presets: [
          ["@babel/preset-typescript", { isTSX: true, allExtensions: true }],
          ["@babel/preset-react", { runtime: "automatic" }],
        ],
      },
      classNameSlug: (hash) => hash,
      evaluate: true,
    }),
  ],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/ingestion.v1.IngestionService": {
        target: API_PROXY_TARGET,
        changeOrigin: true,
      },
      "/metrics.v1.MetricsService": {
        target: API_PROXY_TARGET,
        changeOrigin: true,
      },
      "/auth.v1.AuthService": {
        target: API_PROXY_TARGET,
        changeOrigin: true,
      },
    },
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
