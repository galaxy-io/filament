import path from "node:path";

import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import wyw from "@wyw-in-js/vite";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig(() => {
  const port = process.env.VITE_PORT ? Number.parseInt(process.env.VITE_PORT, 10) : 5173;
  // The Go ConnectRPC server the dev server proxies to (INGESTION_ADDR).
  const apiTarget = process.env.VITE_API_PROXY_TARGET ?? "http://localhost:8080";

  return {
    plugins: [
      tanstackRouter({
        target: "react",
        autoCodeSplitting: true,
      }),
      viteReact(),
      // Compiles Linaria styles for the app's own source only — @galaxy-io/dls
      // ships precompiled CSS (dist + styles.css) and needs no processing.
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
      port,
      strictPort: true,
      // Same-origin in dev: proxy ConnectRPC calls to the Go server so no CORS
      // configuration is needed and dev matches the embedded production setup.
      proxy: {
        "/ingestion.v1.IngestionService": {
          target: apiTarget,
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
      sourcemap: !!process.env.VITE_DEBUG,
      rollupOptions: {
        onwarn(warning, warn) {
          // Ignore PURE comment warnings from plugins
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
  };
});
