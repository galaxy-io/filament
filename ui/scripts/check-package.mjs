import fs from "node:fs";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..");
const DIST = path.join(ROOT, "dist");
const HOST_DIR = path.join(DIST, "host");
const CHECKED_EXTENSIONS = [".js", ".d.ts"];
const SPECIFIER =
  /^\s*(?:import|export)\b[^"';]*?\bfrom\s*["']([^"']+)["']|^\s*import\s*["']([^"']+)["']|\bimport\(\s*["']([^"']+)["']\s*\)/gm;
const NODE_BUILTIN = /^node:/;
const RELATIVE = /^\.{1,2}\//;
const ALIAS = /^@\//;

const pkg = JSON.parse(fs.readFileSync(path.join(ROOT, "package.json"), "utf8"));
const declared = new Set([
  pkg.name,
  ...Object.keys(pkg.dependencies ?? {}),
  ...Object.keys(pkg.peerDependencies ?? {}),
]);

const getPackageName = (specifier) =>
  specifier
    .split("/")
    .slice(0, specifier.startsWith("@") ? 2 : 1)
    .join("/");

const isChecked = (file) => CHECKED_EXTENSIONS.some((extension) => file.endsWith(extension));

const getFileProblems = (file) => {
  const source = fs.readFileSync(path.join(DIST, file), "utf8");
  return [...source.matchAll(SPECIFIER)].flatMap(([, ...groups]) => {
    const specifier = groups.find(Boolean) ?? "";
    if (ALIAS.test(specifier)) {
      return [`${file}: unresolved alias ${specifier}`];
    }
    if (RELATIVE.test(specifier) || NODE_BUILTIN.test(specifier)) {
      return [];
    }
    const name = getPackageName(specifier);
    return declared.has(name) ? [] : [`${file}: ${name} is not a dependency or peer`];
  });
};

const problems = [
  ...(fs.existsSync(HOST_DIR)
    ? [`${path.relative(ROOT, HOST_DIR)} exists: library code reached src/host`]
    : []),
  ...fs
    .readdirSync(DIST, { recursive: true })
    .map(String)
    .filter(isChecked)
    .flatMap(getFileProblems),
];

if (problems.length > 0) {
  console.error(problems.join("\n"));
  process.exit(1);
}

console.log(`${pkg.name}: dist imports only declared packages`);
