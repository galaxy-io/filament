import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";

import { API_PROXY } from "../../vite.config.ts";

const FIXTURE_PORT = 5176;
const FIXTURE_CACHE_DIR = "../../node_modules/.vite-host-fixture";

export default defineConfig({
  root: import.meta.dirname,
  cacheDir: FIXTURE_CACHE_DIR,
  plugins: [viteReact()],
  server: {
    port: FIXTURE_PORT,
    strictPort: true,
    proxy: API_PROXY,
  },
  build: {
    write: false,
  },
});
