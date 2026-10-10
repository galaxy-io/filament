import fs from "node:fs";
import path from "node:path";

import viteReact from "@vitejs/plugin-react";
import wyw from "@wyw-in-js/vite";
import { defineConfig } from "vite";
import dts from "vite-plugin-dts";

interface PackageExportTarget {
  types: string;
  import: string;
}

interface PackageManifest {
  exports: Record<string, string | PackageExportTarget>;
}

const SOURCE_EXTENSIONS = [".tsx", ".ts"];

const PACKAGE: PackageManifest = JSON.parse(
  fs.readFileSync(path.resolve(import.meta.dirname, "package.json"), "utf8"),
);

const getEntrySource = (target: string) => {
  const stem = path.resolve(
    import.meta.dirname,
    target.replace(/^\.\/dist\//, "src/").replace(/\.js$/, ""),
  );
  const source = SOURCE_EXTENSIONS.map((extension) => `${stem}${extension}`).find((file) =>
    fs.existsSync(file),
  );
  if (!source) {
    throw new Error(`No source for export target ${target}`);
  }
  return source;
};

const ENTRIES = Object.values(PACKAGE.exports).flatMap((target) =>
  typeof target === "string" ? [] : [getEntrySource(target.import)],
);

const isPackageImport = (id: string) => !/^(\.|\/|@\/|\0)/.test(id) && !id.endsWith(".css");

export default defineConfig({
  plugins: [
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
    dts({ tsconfigPath: "./tsconfig.lib.json", entryRoot: "src" }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  publicDir: false,
  build: {
    outDir: "dist",
    target: "esnext",
    minify: false,
    cssCodeSplit: false,
    lib: {
      entry: ENTRIES,
      formats: ["es"],
      cssFileName: "styles",
    },
    rolldownOptions: {
      external: (id, _importer, isResolved) => !isResolved && isPackageImport(id),
      output: {
        preserveModules: true,
        preserveModulesRoot: "src",
        entryFileNames: "[name].js",
      },
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
