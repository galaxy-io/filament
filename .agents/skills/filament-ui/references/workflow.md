# Workflow

How a feature goes from a proto to a verified screen in Filament, and the commands that gate it. Follow the steps in order. Each one is small, and skipping one is where most rework comes from.

## 0. Start from what exists

- Find the nearest existing screen in [screens.md](./screens.md) and open it. New screens copy an existing shape.
- Read the DLS stories on [storybook.getgalaxy.io](https://storybook.getgalaxy.io) when the layout is not obvious, especially the `Example:` and `Do / Don't` stories, and decide the layout from them.
- Check `ui/src/api/queries/` for the resource. If the RPC is already wrapped, reuse the hook.

## 1. Server and protos

- Add or change the RPC in `protos/<service>/v1/*.proto` and the Go handler. Prefer an entity-scoped Get RPC over making the client find one entity in a list.
- `cd ui && pnpm codegen`, then commit `ui/src/gen/`. A stale `gen/` drops new request fields silently.
- A new service gets a proxy entry in `API_PROXY` in `ui/vite.config.ts`, and a line in the host contract in `ui/README.md`.

## 2. Query layer

- Add or extend `src/api/queries/<resource>.ts` following [data.md](./data.md). Input helper, key factory only if invalidated, plain hook, suspense hook if a page reads it, infinite hooks if the RPC pages, mutations with their invalidations.
- Decide polling in the hook, `staleTime: Infinity` for catalogs, `PROBE_QUERY_OPTIONS` for one-shot probes.

## 3. Route

- Add the schema to `module/schemas.ts`, a read hook to `module/hooks.ts`, a `FilamentPath` member and a `*RouteOptions` in `module/routes.tsx` ([routing.md](./routing.md)).
- Add the file route under `src/host/routes/_app/_filament/` that spreads the options, and the same route in `fixtures/host/router.tsx`.
- If hosts may mount the page, add `./pages/<Page>` to `exports` in `ui/package.json`.
- Run `pnpm dev` once or `pnpm build` so the plugin regenerates `src/host/routeTree.gen.ts`, and commit it.
- A new top-level destination is a `FilamentNavItem` with its map entries in `module/nav.ts`. A new overlay is a `Flow` member, its params in `filamentLayoutSearchSchema`, and a line in `FilamentLayout`.

## 4. Page and components

- `pages/<feature>/<Name>Page.tsx` composes `PageLayout` ([screens.md](./screens.md)) and fetches with suspense hooks. Overlays and dashboard panels use plain hooks.
- Each surface gets its own folder with feature-prefixed components and `constants.ts`, `types.ts`, `utils.ts` as needed ([architecture.md](./architecture.md)).
- Forms follow [forms.md](./forms.md). Dashboards follow [dashboards.md](./dashboards.md).
- Design every state. Loading (`PendingLayout` or a `Skeleton`), empty (`EmptyLayout` with the action that fills it), filtered-empty, error (`ErrorLayout` with a way out), permission (hidden, not disabled), deleted, unsaved.
- Pick DLS components with the decision tables in [components.md](./components.md) and style with tokens only ([tokens.md](./tokens.md)). One `PRIMARY` button per view.

## 5. Gates

From `ui/`, with pnpm (never npm).

```bash
pnpm check
```

It runs, in order, `typecheck`, `typecheck:lib`, `lint:check`, `format:check`, `build`, `build:lib`, `check:package` and `check:host`. After committing, confirm the route tree is current.

```bash
pnpm build && git diff --exit-code src/host/routeTree.gen.ts
```

From the repository root, `just ui-check` runs a frozen install, `pnpm check` and the route-tree diff. CI runs the same recipe.

| Gate | Fails on |
|---|---|
| `typecheck` | the standalone and the module |
| `typecheck:lib` | library code that imports `src/host` (`tsconfig.lib.json` excludes it) |
| `lint:check`, `format:check` | Biome, including import order, `@/host/**` imports outside `src/host`, self-imports by package name, hex, margins, raw text tags, barrels, `import type` |
| `build`, `build:lib` | the standalone (`ui/build/`) and the module (`ui/dist/`) |
| `check:package` | `publint --strict`, `dist/host`, an unresolved `@/` alias or an undeclared bare import in `dist/` |
| `check:host` | the fixture host in `fixtures/host/` type-checking and building against the package by name |

`pnpm format` and `pnpm lint` fix what they can.

### Greps

Run from `ui/src`. Each must print nothing.

```bash
rg -n 'class\*="gx-' --glob '!gen/**' .
rg -n 'as never|as unknown as|@ts-ignore' --glob '!gen/**' --glob '!module/hooks.ts' .
rg -n '(^|\s)(//|/\*|\{/\*)' -g '*.{ts,tsx,css}' --glob '!gen/**' --glob '!**/routeTree.gen.ts' . | rg -v 'biome-ignore|https?://|"[^"]*//[^"]*"|`[^`]*//[^`]*`'
rg -n '^const [A-Z][a-z]\w* = \(' -g '*.tsx' --glob '!gen/**' .
rg -n '^const [A-Z]\w* = memo\(|^(export )?(default )?function [A-Z]|React\.FC' -g '*.{ts,tsx}' --glob '!gen/**' .
rg -n '(^|\s)(//|/\*)' ../vite.config.ts ../vite.lib.config.ts ../tsconfig*.json ../scripts | rg -v 'biome-ignore|https?://'
rg -n '(^|\s)(//|/\*)' ../fixtures | rg -v 'biome-ignore|@ts-expect-error|https?://'
rg -n 'BigInt\.prototype|toJSON' --glob '!gen/**' .
rg -n ':global' --glob '!host/**' --glob '!gen/**' .
rg -n 'useSearch\(|useParams\(|useNavigate\(' --glob '!host/**' --glob '!module/**' --glob '!gen/**' .
rg -n 'from: "/_app|to: "/' --glob '!host/**' --glob '!gen/**' .
rg -n '"\./(pipelines|sources|sinks|observability|settings)' --glob '!module/paths.ts' --glob '!host/**' --glob '!gen/**' .
for f in connections pipelines observability settings; do rg -n --pcre2 "from \"@/pages/(?!$f/)" "pages/$f"; done
rg -n 'from "@/pages/' api components layouts hooks utils
rg -n 'from "@/pages/[^"]*Page"' module/routes.tsx | rg -v 'PipelineNotFoundPage'
rg -n 'use(Layout)?Effect\(' -g '*.tsx' --glob '!gen/**' .
rg -n '^\s+[a-z-]+: [^$]*\b[0-9]+px' -g '*.tsx' --glob '!gen/**' .
```

In order, they catch DLS internal selectors, escape-hatch casts, comments, components not typed `FC`, `function` components and `React.FC`, comments in config, scripts and the fixture, the old BigInt patch, global CSS outside the host, router hooks outside `module/`, absolute paths and route ids in library code, path strings outside `FilamentPath`, cross-feature imports, shared code importing a page, the route module importing pages, effects in components, and raw pixel values in templates.

## 6. Verify in the browser

- `just dev` runs the API on 8080 and Vite on 5173. Do not restart a dev server someone else is using.
- Start your own. `.agents/launch.json` names `filament-ui-verify` (Vite on 5175 with `--mode verify`, which uses its own `node_modules/.vite-verify` cache) and `filament-host-fixture` (the fixture on 5176, after `pnpm build:lib`). `API_PROXY_TARGET` points either proxy at another API.
- Every Vite you start needs its own port and its own `cacheDir`. A second Vite in `ui/` without one overwrites `node_modules/.vite`, the cache a running `just dev` serves from.
- A Go change needs its own server too, for example `SERVER_ADDR=:8090` plus a Vite whose proxy points at it. There is no HMR through a proxy, so reload after each edit.
- Check both themes, the loading, empty, filtered-empty and error states, keyboard focus on new controls, and the console.
- Check the URL after every interaction. It should hold only what the user changed, with defaults absent.
- Leave nothing unsaved. If you edited a pipeline on the canvas to verify, use Undo before you leave.

## Things that look like bugs and are not

| Symptom | Cause | Fix |
|---|---|---|
| DLS components render unstyled after a DLS bump | the Vite prebundle kept JS from the previous DLS while `styles.css` is new, so class hashes miss | delete that Vite's cache directory and restart it |
| Editing a constant used in a `styled` template changes nothing | Linaria does not watch the constant's module | touch the component file |
| A `SelectInput` "won't open" | DLS disables a select with one option or none | check the options you passed |
| A static `Chip` renders as a button | it is wrapped in `Tooltip`, which injects `onClick` | pass `tooltip` on the Chip |
| The canvas looks broken or exit animations linger in a hidden Browser pane | `requestAnimationFrame` does not fire while hidden | take a screenshot to flush a frame, or bring the pane forward |
| Queries stop retrying in a hidden Browser pane | TanStack pauses retries while the window is unfocused | focus the pane, or wait |
| A run's activity feed goes stale with `ERR_INCOMPLETE_CHUNKED_ENCODING` | the run ended and the server closed the stream | not a UI bug |
| An auth outage sends users to the login page | the gate redirected on a code other than `Unauthenticated` | the server must return `Unavailable`, and the gate redirects only on `Unauthenticated` |
| The console logs an error while a not-found or error page renders | React 18 logs errors caught by a boundary in development | expected in dev |

## Before you hand it over

- [ ] The screen matches an existing shell and reuses its pieces.
- [ ] Every component is `FC`, every name carries its feature prefix, one export per file, no barrels, no comments, no `useEffect` in `.tsx`.
- [ ] Types come from the protos, enum maps are exhaustive `Record`s in `constants.ts`, `UNSPECIFIED` is handled by behaviour.
- [ ] View state is in the URL, read through a `module/hooks.ts` read hook, written by spreading `prev`. No absolute paths in library code.
- [ ] Data goes through `src/api/queries`, invalidation lives in the mutation hook, loading gates on `isLoading`.
- [ ] Loading, empty, filtered-empty and error are all designed and all different.
- [ ] One `PRIMARY` per view, destructive actions confirmed by name, toasts name the entity.
- [ ] Both themes checked, keyboard checked, icon-only controls labelled.
- [ ] `pnpm check` passes, every grep is empty, and `src/host/routeTree.gen.ts` is committed.
