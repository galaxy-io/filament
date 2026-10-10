# Filament UI

Web UI for Filament, built with Vite + React + [@galaxy-io/dls](https://www.npmjs.com/package/@galaxy-io/dls) (the Galaxy Design Language System), talking to the ConnectRPC `IngestionService`.

## Develop

```bash
just infra   # postgres + nats (Compose; CONTAINER_ENGINE=podman selects Podman)
just dev     # control-plane + server (:8080) + this UI (:5173)
```

Or run the pieces individually: `just server`, `just control-plane`, and `just ui`.

The dev server proxies `/ingestion.v1.IngestionService`, `/metrics.v1.MetricsService` and `/auth.v1.AuthService` to `http://localhost:8080` (the port the justfile server recipe binds), so the app is same-origin in dev and prod alike. Set `API_PROXY_TARGET` to proxy to another local server, or `VITE_API_URL` to point the app at a remote Filament instance.

## Build & embed

The server binary embeds this app. `just binaries` builds `build/` and compiles
`cmd/server` with `-tags embedui`. To build those pieces from the repository
root:

```bash
just ui-build                                     # tsc + vite build → ui/build/
GOWORK=off go build -C cmd/server -tags embedui . # binary serving API + UI
```

Without `-tags embedui`, `ui.Handler()` serves a placeholder page and `go build ./...` needs no Node toolchain.

## Module and host

The same `src/` tree also builds `@galaxy-io/filament`, a module a Galaxy host mounts under any path.

```bash
pnpm build:lib       # vite.lib.config.ts → dist/
pnpm check:package   # publint and the dist import scan
pnpm check:host      # type-check and build the fixture host against the package
```

`src/host/` is standalone-only and never ships. `src/module/` is the module's surface. The public API is the `exports` map in `package.json`.

| Subpath | What |
|---|---|
| `./paths` | `FilamentPath` and `createFilamentHref` |
| `./routes` | one `*RouteOptions` per page, plus `filamentLayoutRouteOptions` and `filamentNotFoundRouteOptions` |
| `./FilamentLayout` | the shell, its sidebar and the URL-driven drawer and modals |
| `./commands` | `useFilamentCommandItems({ base, transport })` for a host command palette |
| `./pages/<Page>` | `ObservabilityPage`, `PipelinesPage`, `PipelinePage`, `PipelineCanvasPage`, `PipelineHistoryPage`, `PipelineSettingsPage`, `SourcesPage`, `SinksPage`, `NotFoundPage` |
| `./styles.css` | every class the module renders, all prefixed `filament-` |

`fixtures/host/` is the reference host and the executable copy of this contract. A host does the following.

1. **Install** `@galaxy-io/filament` and every peer dependency in `package.json`. The host `tsconfig` uses `"moduleResolution": "bundler"`, which every Vite host does. The host bundler replaces `process.env.NODE_ENV`, which Vite and webpack do.
2. **Import the CSS once** at the app entry, in this order. `@galaxy-io/dls/styles.css`, `tokens.css`, `fonts.css`, then `@galaxy-io/filament/styles.css`. Filament overrides the DLS at equal specificity, so its file comes last.
3. **Mount the providers** above the router. `GalaxyProvider` owns the theme (Filament has no theme API), then `QueryClientProvider` and `RouterProvider`.
4. **Give Filament a transport.** Filament reads the connect-query transport from context. A host whose global transport already reaches Filament's services needs nothing more. Otherwise nest a `TransportProvider` in the layout route's component. Create the transport once, at module scope, and pass the same instance to `useFilamentCommandItems` when the palette lives outside the mount, so the palette and the pages share cache entries. The module takes no `baseUrl`. Only the host adds auth interceptors or picks the wire format.
5. **Add the routes.**
   - A layout route `{ ...filamentLayoutRouteOptions, component: Layout }`.
   - One child per page, `{ ...pipelinesRouteOptions, component: PipelinesPage }`, with the page imported in that file.
   - A `$` splat `{ ...filamentNotFoundRouteOptions, component: NotFoundPage }`.
   - Index redirects (`/` → `observability`, `pipelines/$id/` → `canvas`) as host files with absolute paths.
   - No `defaultPendingComponent`, `defaultErrorComponent` or `defaultNotFoundComponent` is needed. The route options carry Filament's own.
6. **Own the query client.** Filament works with TanStack's defaults. Skip retries for `NotFound`, `PermissionDenied`, `Unauthenticated`, `InvalidArgument` and `Unimplemented`, or a missing pipeline retries for about 7s before its not-found page. A `staleTime` of 30s is recommended.
7. **Proxy** `/ingestion.v1.IngestionService`, `/metrics.v1.MetricsService` and `/auth.v1.AuthService` to the Filament API at the transport's `baseUrl`. When another product also serves `metrics.v1.MetricsService`, put one product behind a prefix.
8. **Give the layout a box.** `FilamentLayout` fills its parent, so the parent needs a definite height (a flex child with `min-height: 0`).

## Codegen

TypeScript clients are generated from the protobuf definitions under `../api/`
into `src/gen/` (checked in):

```bash
cd ui && pnpm codegen   # buf generate
```

Re-run after changing the protos. `src/host/routeTree.gen.ts` is generated by the TanStack Router plugin during `pnpm dev` / `pnpm build` and is also checked in.

## Conventions

Everything in `src/` follows the `filament-ui` skill at `.agents/skills/filament-ui` (DLS components by subpath, Linaria with DLS tokens, no margins, no barrels, feature-prefixed files, URL-backed view state, the query layer in `src/api/queries`). Read its `SKILL.md` before adding a screen, and update its references when a convention changes.
