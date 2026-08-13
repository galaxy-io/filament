import path from "node:path";

import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import wyw from "@wyw-in-js/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
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
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/metrics.v1.MetricsService": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/auth/config": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/auth/login": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/auth/register": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/auth/invite": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/auth/members": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    target: "esnext",
    rollupOptions: {
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
});
