# Workflow

How a feature goes from a proto to a verified screen in Filament, and the commands that gate it. Follow the steps in order. Each one is small, and skipping one is where most rework comes from.

## 0. Start from what exists

- Find the nearest existing screen in [screens.md](./screens.md) and open it. New screens copy an existing shape, they do not invent one.
- Read the DLS stories for the components you plan to use when the layout is not obvious. They are on [storybook.getgalaxy.io](https://storybook.getgalaxy.io), especially the `Example:` and `Do / Don't` stories. Decide the layout yourself from them.
- Check `ui/src/api/queries/` for the resource. If the RPC is already wrapped, reuse the hook.

## 1. Server and protos

- Add or change the RPC in `api/<service>/v1/*.proto` and the Go handler. Prefer an entity-scoped Get RPC over making the client derive one entity from a list.
- `cd ui && pnpm codegen`, then commit `ui/src/gen/`. A stale `gen/` drops new request fields silently.
- A new service gets a proxy entry in `ui/vite.config.ts`.

## 2. Query layer

- Add `src/api/queries/<resource>.ts` or extend it, following the contract in [data.md](./data.md). Input factory, key factory, options factory, plain hook, suspense hook, infinite hooks if the RPC pages, mutations with their invalidations.
- Decide polling in the hook (`refetchInterval` as a function of the data), `staleTime: Infinity` for catalogs, `PROBE_QUERY_OPTIONS` for one-shot probes.

## 3. Route

- Add the route file under `src/routes/` ([routing.md](./routing.md)). `validateSearch` with every param `.optional().catch(undefined)`, `remountDeps` for a list page, `component` pointing at the `*Page.tsx`.
- Run `pnpm dev` once (or `pnpm build`) so the plugin regenerates `routeTree.gen.ts`, and commit it.
- A new top-level destination also needs an entry in `MAIN_NAVBAR_ITEMS` (`layouts/main/MainLayoutNavbar.tsx`). A new app-wide overlay needs a `Flow` member, its params on `/_app`, and a line in `AppLayout`.

## 4. Page and components

- `pages/<feature>/<Feature>Page.tsx` composes a shell from [screens.md](./screens.md) and fetches with suspense hooks (plain hooks for overlays and dashboards).
- Each surface gets its own folder with feature-prefixed components, `constants.ts`, `types.ts`, `utils.ts` as needed ([architecture.md](./architecture.md)).
- Forms follow [forms.md](./forms.md). Dashboards follow [dashboards.md](./dashboards.md).
- Design every state before you call it done. Loading (`PendingLayout` or a `Skeleton`), empty (`EmptyLayout` with the action that fills it), filtered-empty, error (`ErrorLayout` with a way out), permission (hidden, not disabled), deleted (`Chip label="Deleted"`), stale or unsaved (`Chip variant={ERROR}`).
- Pick DLS components with the decision tables in [components.md](./components.md) and style with tokens only ([tokens.md](./tokens.md)). One `PRIMARY` button per view.

## 5. Gates

Run from `ui/`. These are the only automated checks, so run them after every meaningful change.

```bash
pnpm typecheck
```

```bash
pnpm lint:check
```

```bash
pnpm format:check
```

```bash
pnpm build
```

```bash
git diff --exit-code src/host/routeTree.gen.ts
```

`pnpm format` and `pnpm lint` fix what they can. Biome enforces import order, no hex, no margins, no raw text tags, no barrels, `import type`.

## 6. Verify in the browser

- The dev server is `pnpm dev` in `ui/` on 5173, proxying the Connect services to the Go server on 8080. The repo's `.agents/launch.json` names it `filament-ui`, and `filament-ui-verify` starts a second Vite on 5175 for checking without touching a running server. Do not restart a dev server someone else is using. Start your own on a free port.
- A Go change needs its own server too. `SERVER_ADDR=:8090` for the API plus a Vite instance whose proxy points at it. There is no HMR through a proxy, so reload after each edit.
- Check both themes (the `ThemeSwitcher` in the account menu), the empty state, the loading state (throttle the network or reset the query), the error state, keyboard reachability, and that icon-only controls have a tooltip.
- Check the URL after every interaction that changes view state. It should contain only what the user changed, with defaults absent.
- Leave nothing unsaved. If you edited a pipeline on the canvas to verify, use Undo in its navbar before you leave.

## Things that look like bugs and are not

| Symptom | Cause | Fix |
|---|---|---|
| DLS components render unstyled, modal padding and footer compute to 0 | `node_modules/.vite` kept JS from a previous DLS build while `styles.css` is new, so Linaria class hashes miss | `rm -rf ui/node_modules/.vite` and restart Vite, or run a throwaway Vite with its own `cacheDir` and `--force` |
| Editing a constant used in a `styled` template changes nothing after reload | Linaria does not watch the constant's module | touch or edit the component file |
| A `SelectInput` "won't open" | DLS disables a select with one option or none | check the options you passed |
| A static `Chip` renders as a button with a pointer | it is wrapped in `Tooltip`, which injects `onClick` | pass `tooltip` on the Chip |
| A deep link with `?runId=` loses the param | `InfiniteTable` prunes expanded ids not in the loaded page | load the row first |
| The drawer shows "No pipelines" for a moment while closing | content read a search param that is already gone during the exit animation | pass the id as a prop and hold it with `useRetainedWhileClosed` |
| A per-cell `isLoading` is never true | the page already suspends on the same query key | remove the dead loading branch |
| The pipeline canvas looks broken in a backgrounded Browser pane | `requestAnimationFrame` never fires while hidden, so xyflow never re-measures and framer exits never finish | take a screenshot to flush a frame, or bring the pane forward |
| A run's activity feed goes stale with `ERR_INCOMPLETE_CHUNKED_ENCODING` | the run ended and the server closed the stream, nothing reconnects by design | not a UI bug |
| An invalidated query served by a route loader still shows old data | `ensureQueryData` ignores `staleTime` | there are no loaders by convention, and `revalidateIfStale: true` if one is ever added |
| An auth outage sends users to the login page | the gate redirected on a code other than `Unauthenticated` | the server must return `Unavailable`, and the gate only redirects on `Unauthenticated` |

## Before you hand it over

- [ ] The screen matches an existing shell and reuses its layout pieces.
- [ ] Every name carries its feature prefix, one export per file, no barrels, no comments, no `useEffect` in components.
- [ ] Types come from the protos (indexed access), enum maps are exhaustive `Record`s, `UNSPECIFIED` is handled by behaviour.
- [ ] View state is in the URL with `.optional().catch(undefined)`, read with `from`, written by spreading `prev`.
- [ ] Data goes through `src/api/queries`, invalidation lives in the mutation hook, loading gates on `isLoading`.
- [ ] Loading, empty, filtered-empty and error are all designed and all different.
- [ ] One `PRIMARY` per view, destructive actions confirmed by name, toasts say what happened and name the entity.
- [ ] Run status marks are squares from the hue map. Status color only for status, palette color only for category.
- [ ] Both themes checked, keyboard checked, icon-only controls labelled.
- [ ] `pnpm typecheck`, `pnpm lint:check`, `pnpm format:check`, `pnpm build` pass and `routeTree.gen.ts` is committed.
