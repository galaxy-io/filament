# Architecture

How Filament's `ui/` is put together, and the house conventions every new file follows. The DLS references in this folder say what the components are. This page says where code goes and how it is shaped. The last section is the recipe for building any Galaxy app the same way.

## Stack

| Concern | Choice | Where |
|---|---|---|
| Build | Vite 8 (rolldown), `galaxyDls` from `@galaxy-io/dls/vite` for Linaria and the theme boot script, `@tanstack/router-plugin` with `autoCodeSplitting` | `ui/vite.config.ts` (standalone), `ui/vite.lib.config.ts` (library) |
| UI | React 18, `@galaxy-io/dls` 2.10, Phosphor icons | everywhere under `ui/src` |
| Routing | TanStack Router. File routes in the standalone, route options exported to hosts | `ui/src/host/routes/`, `ui/src/module/routes.tsx` |
| Data | ConnectRPC over `@connectrpc/connect-query` + TanStack Query v5 | `ui/src/api/queries/` |
| Types | protobuf-es generated from `protos/**` | `ui/src/gen/` (checked in, regenerate with `pnpm codegen`) |
| Styling | Linaria `styled` reading DLS tokens through `t` | per component file |
| Lint and format | Biome extending `@galaxy-io/dls/biome` | `ui/biome.json` |
| URL state | zod schemas, parsed by the module read hooks | `ui/src/module/schemas.ts`, `ui/src/module/hooks.ts` |
| Branching on enums | `ts-pattern` `match().exhaustive()` | where a component picks a branch per enum member |
| Canvas | `@xyflow/react` | `pages/pipelines/canvas/` |
| Package checks | `publint --strict` + `scripts/check-package.mjs` | `pnpm check:package` |

There is no test runner. `pnpm check` runs every gate (see [workflow.md](./workflow.md)).

## One tree, two builds

`ui/src` builds two things.

| Build | Command | Output | Who uses it |
|---|---|---|---|
| The standalone app | `pnpm build` | `ui/build/` | the Go server embeds it |
| The module | `pnpm build:lib` | `ui/dist/` | a Galaxy host installs `@galaxy-io/filament` |

`src/host/**` is standalone-only. Everything else is library code, and library code never imports `src/host`. The boundary is written in three places that must agree.

- `tsconfig.lib.json` excludes `src/host/**`, so `pnpm typecheck:lib` fails on a leak.
- Biome bans `@/host/**` imports in `src/**` outside `src/host/**`.
- Biome bans importing the package by its own name (`@galaxy-io/filament`) inside `src/`.

The package surface is the `exports` map in `ui/package.json`, and nothing else is public.

| Subpath | Source | What |
|---|---|---|
| `./paths` | `module/paths.ts` | `FilamentPath` and `createFilamentHref` |
| `./routes` | `module/routes.tsx` | one `*RouteOptions` per exported page, the layout and the splat |
| `./FilamentLayout` | `module/FilamentLayout.tsx` | the shell, the sidebar and the URL-driven overlays |
| `./commands` | `module/commands.ts` | `useFilamentCommandItems({ base, transport })` for a host command palette |
| `./pages/<Page>` | `pages/**/<Page>.tsx` | `ObservabilityPage`, `PipelinesPage`, `PipelinePage`, `PipelineCanvasPage`, `PipelineHistoryPage`, `PipelineSettingsPage`, `SourcesPage`, `SinksPage`, `NotFoundPage` |
| `./styles.css` | the build | every `filament-*` class plus xyflow's stylesheet |

`vite.lib.config.ts` derives its entries from `exports`, so a new subpath is one line in `package.json`. `TeamPage` and `ServiceAccountsPage` stay standalone-only because the host owns identity.

## Folder map

```
ui/src
├── host/                    standalone-only, never imported by library code
│   ├── main.tsx · App.tsx · router.tsx · style.css · routeTree.gen.ts
│   ├── api/                 transport, queryClient, TransportQueryClientProvider
│   ├── auth/                session types, auth hooks, invite and login helpers
│   ├── layouts/app/         AppLayout, the account menu, the command palette
│   ├── layouts/auth/        the login / register / invite shell
│   ├── pages/auth/          LoginPage, RegisterPage, InvitePage
│   └── routes/              file routes that spread the module's route options
├── module/                  the package surface and the contract every page relies on
│   ├── FilamentLayout.tsx · FilamentErrorComponent.tsx · FilamentPendingComponent.tsx
│   ├── paths.ts · routes.tsx · commands.ts · nav.ts
│   ├── schemas.ts           every search and params schema
│   ├── hooks.ts             the URL read and write hooks
│   └── constants.ts · types.ts
├── pages/                   one folder per feature
│   ├── NotFoundPage.tsx
│   ├── pipelines/           PipelinesPage, PipelinePage, PipelineCanvasPage, PipelineHistoryPage, PipelineSettingsPage
│   ├── connections/         ConnectionsPage, SourcesPage, SinksPage
│   ├── observability/       ObservabilityPage
│   └── settings/            TeamPage, ServiceAccountsPage (standalone routes only)
├── components/              pieces two or more features use
│   ├── connections/         ConnectorTile, ConnectorLogoTile, ConnectionKindChip, …
│   ├── pipelines/           PipelineCard, PipelineFlow, PipelineName, …
│   ├── runs/                PipelineRunStatus, PipelineRunStatusSwatch, PipelineRunDuration, …
│   ├── fields/              schema-driven form fields (Field, FieldString, FieldSecret, …)
│   └── BaseHeader · DocsLink · EmptyGraphic · IconTile · KeyValueList · ListSearch · RouterLink
├── layouts/main/            MainLayout (DLS AppFrame) and MainLayoutSidebar
├── api/queries/             one file per resource
├── hooks/                   app-generic hooks (useConfirm, useOverlaySession, useOverlayRecord)
├── utils/                   app-generic helpers (errors, format, naming, runs, select, sort, validation)
├── constants.ts             IS_DEBUG, docs and community URLs, debounce timings, ACTIVE_RUN_STATUSES
└── gen/                     generated protobuf clients and messages
```

Inside a feature folder the shape repeats.

```
pages/pipelines
├── PipelinesPage.tsx                 one route = one *Page.tsx at the feature root
├── PipelineSettingsPage.tsx
├── constants.ts · utils.ts           feature-wide helpers and constants
├── hooks/                            feature hooks (usePipelinePreviewVersion, usePipelineCanvasNavigate, …)
├── settings/                         one folder per sub-route
│   ├── PipelineSettingsPageGeneral.tsx
│   └── PipelineSettingsPageDanger.tsx
├── history/
├── components/                       one folder per surface
│   ├── header/                       PipelinePageHeading, PipelinePageTabs, PipelinePageActions, …
│   ├── table/                        PipelinesTable + columns/
│   ├── create/                       CreatePipelineModal + its provider + steps/
│   └── schedule/ notifier/ worker/ resource/ node/ transform/
└── canvas/                           the editor, with its own nodes/ edges/ panel/ providers/ routes/
```

- A route screen is `pages/<feature>/<Name>Page.tsx`. A sub-route's pieces go in `pages/<feature>/<subroute>/`, surfaces in `pages/<feature>/components/<surface>/`.
- A component moves to `src/components/<domain>/` only when a second feature imports it. No `pages/<a>` file imports from `pages/<b>`.
- `api/`, `components/`, `layouts/`, `hooks/` and `utils/` never import from `pages/`.

## File contract

- **Every component is typed `FC`.** `import type { FC, PropsWithChildren } from "react"`, never `React.FC`, never `function` components, never an annotated return type.
  - The default is `const X: FC<XProps> = ({ a, b = DEFAULT_B }) => { … };` then `export default X;`. Defaults go in the destructuring.
  - A component with no props is `const X: FC = () => …`. A component with children is `FC<PropsWithChildren<XProps>>`.
  - With `memo` (xyflow nodes and virtualized rows only), declare `const X: FC<XProps>` and export `export default memo(X);`.
  - Two exceptions only. `forwardRef` components (`RouterLink`) stay `forwardRef<Element, XProps>(…)`. Generic components (`ObservabilityTimeseriesWidget`, `PipelineNotifierTable`) stay `const X = <T extends …>({ … }: XProps<T>) => …` because `FC` cannot carry a type parameter.
- **Props are a non-exported `interface XProps`** directly above the component. The only public props interface is `FilamentLayoutProps`. xyflow node props alias `NodeProps<…>` in `canvas/nodes/types.ts`.
- **One export per file, default exported, filename equals the export.** The only extra named exports allowed are the component's own size or variant enum, or a data interface its callers build. Extra components get their own files.
- **File order.** Imports, private constants and maps, styled components, `XProps`, `XState` with `DEFAULT_STATE`, the component, the default export. Inside a component, hooks first, then derived values, then `handleX` functions, then JSX.
- **Zero comments.** The only comment allowed in `ui/` TypeScript, TSX, CSS and config is a `biome-ignore` directive, plus the fixture's one `@ts-expect-error`. Encode a constraint in a name instead.
- **No `index.ts` barrels.** Import the file.
- **Hooks are named exports** in `hooks/useX.ts`, never default, prefixed with the feature. App-generic hooks stay unprefixed.
- **No effects in `.tsx`.** An effect lives only in a named `hooks/useX.ts` that syncs one external system (`usePipelineCanvasSelectionReveal`, `usePipelineRunNow`).

## Per-folder files

| File | Holds |
|---|---|
| `types.ts` | string `enum`s (`MEMBER = "MEMBER"`) and shared interfaces. No const-object enums. |
| `constants.ts` | SCREAMING_SNAKE constants with the surface prefix, and every map. Enum-keyed maps are exhaustive `Record<X, Y>` with an `UNSPECIFIED` entry, never `Partial`. A map that may have no value types its values `Y \| undefined`. |
| `utils.ts` | pure `export const verbNoun = (…) =>` functions. No maps, hooks or React. |

## Naming

| Thing | Shape | Example |
|---|---|---|
| Constant | `SCREAMING_SNAKE`, surface prefixed | `PIPELINE_CANVAS_PANEL_WIDTH`, `SETTINGS_INVITE_PATH_PREFIX` |
| Lookup | `X_TO_Y_MAP`, an exhaustive `Record` | `PIPELINE_RUN_STATUS_TO_HUE_MAP`, `READ_MODE_TO_WRITE_MODES_MAP` |
| Enum | `PascalCase` with `SCREAMING` members whose value equals the name | `enum Flow { CREATE_PIPELINE = "CREATE_PIPELINE" }` |
| Helper | `create`, `map`, `get`, `format`, `is`, `has`. `parse` and `serialize` only for codec pairs. Never `build`, `to` or `resolve` | `createGetPipelineInput`, `getCompatibleWriteModes`, `formatMemberName` |
| Boolean | `isX`, `hasX`, `canX`, `shouldX`, positive | `isLoading`, `hasChanges`, `canSave` |
| Handler | `handleX` inside the component, `onX` as a prop | `handleSave`, `onClose` |
| Props | `<Component>Props` | `interface PipelinesTableProps` |
| Component state | `<Component>State` + `DEFAULT_STATE` | `PipelineSettingsPageGeneralState` |
| Query hook | `useXQuery`, `useSuspenseXQuery`, `useXInfiniteQuery`, `useXMutation` | `useGetPipelineQuery`, `useDeletePipelineMutation` |
| Module surface | one product prefix | `FilamentLayout`, `FilamentPath`, `useFilament*`, `filament*RouteOptions` |

The prefix is the folder. Everything under `pages/pipelines` is `Pipeline*` or `Pipelines*`, canvas pieces are `PipelineCanvas*`, `pages/settings/components/team/` holds `SettingsTeam*`.

## Imports

Biome sorts imports into groups with a blank line between each. Write them this way and `pnpm format` keeps them.

```tsx
import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { ArrowRightIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";

import { type Connection, ConnectionSchema } from "@/gen/ingestion/v1/connections_pb";

import { CONNECTOR_KIND_TO_NOUN_MAP } from "@/components/connections/constants";

import MainLayout from "@/layouts/main/MainLayout";

import ConnectionForm from "@/pages/connections/components/form/ConnectionForm";

import { useFilamentLayoutSearch } from "@/module/hooks";

import { useGetConnectionQuery } from "@/api/queries/connections";

import { useOverlaySession } from "@/hooks/useOverlaySession";

import { IS_DEBUG } from "@/constants";

import { getErrorMessage } from "@/utils/errors";
```

- The order is React, packages, `@galaxy-io/**`, `@/gen`, `@/components`, `@/layouts`, `@/pages`, `@/module`, `@/host` (host files only), `@/api`, `@/hooks`, `@/constants`, then a catch-all for `@/utils` and relative paths.
- Always the `@/` alias.
- DLS by subpath, the default export plus its enums from the same module. Shared enums come from `@galaxy-io/dls/theme/enums`. Never a barrel.
- `import type` for types. Write `import z from "zod"` and `void navigate(…)`.

## Types come from the protos

- A prop, state field, param or `Record` key that carries a proto field uses an **indexed access type**. `Pipeline["id"]`, `Connection["connector"]`, `Notifier["events"]`. Display strings and generic components stay `string`.
- **Proto messages are state.** Never hand-roll a parallel type. `Pick` from the message when a component needs a subset (`ServiceAccountCredentials = Pick<RotateServiceAccountSecretResponse, "clientId" | "clientSecret">`).
- **Proto enums are used directly.** `z.enum(ConnectorKind)` in a schema, `Record<RunStatus, …>` for a map. Numeric wire values in a URL are fine.
- **`UNSPECIFIED` is handled by behaviour, not types.** A selector treats it as "all kinds", a flow that needs a kind shows the chooser.
- A list of enum members is derived, `Object.values(RunStatus).filter(…)`, not hand typed.

## State doctrine

- **Component state is one object.** `interface XState`, a module `DEFAULT_STATE`, `useState<XState>` and `handleX` setters that spread `prev`. Seed it from data with a lazy `createInitialState`, reset it from outside with `key`.
- **Derived data is computed at render**, with `useMemo` when it is expensive. It is never stored in state.
- **Maps over ternaries and lookups.** An enum-keyed ternary becomes an `X_TO_Y_MAP` in `constants.ts`. Boolean guards and narrowing branches stay as code.
- **Branch per enum member with `match`** from `ts-pattern`, so a new member fails to compile.
- **Shared view state lives in the URL**, not in a context. See [routing.md](./routing.md).
- **Reducer surfaces share one shape.** `XProvider.tsx` + `actions.ts` + `reducer.ts` + `types.ts` + `utils.ts` (with `createInitialXState`). The provider exports `useXState` and `useXActions` and keeps dispatch private. Cases are private `function` helpers behind a delegating `switch`. The model is `pages/pipelines/canvas/providers/canvas/`. See [forms.md](./forms.md).
- **Overlays stay mounted.** They take `isOpen` and their record as props, and `useOverlaySession(isOpen)` gives a key that resets their state on every open.

## Styling inside the app

Everything the DLS references say about tokens applies. The app adds a few habits.

- **Reach for `Flex`, `Box`, `FlexItem` and `Grid` props first,** with numeric scale values (`gap={8}`). A `styled` component appears only for a frame, positioning, a hand-made button or xyflow chrome. Name it for its role (`XWrapper`, `XButton`), keep it unexported, three per file at most.
- **Templates use tokens.** Spacing is `t.space[n]`, named sizes are `${CONSTANT}px`, borders are `HAIRLINE_WIDTH`, transitions are `t.duration.fast`.
- **Hand-made buttons** use `INTERACTIVE_RESET` and `FOCUS_RING` from `@galaxy-io/dls/styles/mixins`. `$`-props are for continuous values. An enum axis is a `Record<Enum, css>`.
- **Scroll** with `FlexItem grow={1} minHeight={0}` › `ScrollArea` › `Box padding`. Never `overflow="auto"`, because the DLS reset hides native scrollbars.
- **Status and series colours** go through the `RoleColor` hue maps into DLS `color` props. App CSS that must match one reads `t.color.mark.*`.
- **Never select a DLS internal class** (`[class*="gx-`). A missing API is a DLS request, not a workaround.
- **No margins, no hex, no inline `style`** except CSS custom properties and virtualizer geometry. Biome's DLS rules fail the build on the rest.
- **Editing a constant used inside a `styled` template does not hot-reload.** Touch the component file to re-transform it.

```tsx
import { styled } from "@linaria/react";

import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const AccountButton = styled.button`
  ${INTERACTIVE_RESET}
  display: flex;
  align-items: center;
  gap: ${t.space[8]};
  width: 100%;
  min-height: ${t.size.control.large};
  padding: ${t.space[4]} ${t.space[8]};
  border-radius: ${t.radius.md};
  transition: background-color ${t.duration.fast} ease-in-out;

  &:hover,
  &[aria-expanded="true"] {
    background-color: ${t.color.background.hovered};
  }

  ${FOCUS_RING}
`;
```

## Minimalism

- No loader, helper hook, lookup function or wrapper that does not earn its place. Delete dead code when you find it. Export only what has a consumer.
- One caller needs a variant of a shared helper. Add a defaulted argument to the helper, do not branch at the call site.
- Shared components own their copy. Callers pass semantic booleans (`hasStoredSecret`), never free-text labels.
- Build the reusable piece where its data already lives, and migrate the existing one-offs in the same change.
- "Boring default" beats clever cache surgery. Invalidate queries, do not seed them.

## Building a Galaxy module

Filament is the worked example for every Galaxy app that ships as a standalone binary and as a module a host mounts. A second app copies this section and renames `filament` and `Filament`.

**Folder shape.** One `src/` tree. `src/host/**` holds everything only the standalone needs: `main.tsx`, the router and its file routes, the transport and query client, auth, the account menu and command palette. `src/module/**` holds the surface every page relies on: the layout, the path enum, the route options, the URL hooks and schemas, the pending and error components and the command items. Features live in `src/pages/<feature>/`, shared pieces in `src/components/<domain>/`.

**Subpaths.** The `exports` map is the whole API and uses the same shape in every app.

```json
{
  "./paths": "…module/paths",
  "./routes": "…module/routes",
  "./<Product>Layout": "…module/<Product>Layout",
  "./commands": "…module/commands",
  "./pages/<Page>": "…pages/<feature>/<Page>",
  "./styles.css": "./dist/styles.css"
}
```

Each target is a `{ types, import }` pair under `dist/`. `vite.lib.config.ts` reads the entries from `exports` and the package name from `package.json`, so neither hard-codes a product.

**Paths.** One `enum <Product>Path` of module-relative strings (`"./pipelines/$id/canvas"`) and `create<Product>Href(path, params)`. These are the only path strings in library code. The module never names an absolute route, so it works under any mount.

**Route options.** Every exported page, the layout and the `$` splat get one `*RouteOptions` object built by a private `create<Product>RouteOptions`. It carries `validateSearch`, `remountDeps` for list params, and the product's own `errorComponent` and `pendingComponent`, so the module looks the same under any host and the host sets no router defaults for it. The layout's options carry `staticData: { <product>: true }`, which lets `use<Product>Base` find the mount.

**URL hooks.** Library code never calls `useSearch`, `useParams` or `useNavigate` directly. It reads through hooks that parse with the zod schema, `useSearch({ strict: false, structuralSharing: true, select: (search) => schema.parse(search) })`, and writes through `use<Product>SearchUpdate` and `use<Product>Navigate`. Defaults apply in the read hook's `select`, never in the schema.

**Class prefix.** `galaxyDls({ prefix: CLASS_PREFIX })` in both Vite configs, with `CLASS_PREFIX` exported once from `vite.config.ts`. Every class the module ships starts with the prefix.

**The fixture.** `ui/fixtures/host/` is a minimal host that imports the package by name (the package self-reference resolves to `dist/`) and mounts every page under one constant `MOUNT_PATH`, with a decoy global transport so a leak of the host's transport shows up. It is the executable copy of the host contract in `ui/README.md`. `pnpm check:host` type-checks and builds it, and it carries one `@ts-expect-error` proving a module route rejects unknown search keys.

**The gates.** `pnpm check` runs `typecheck`, `typecheck:lib`, `lint:check`, `format:check`, `build`, `build:lib`, `check:package` (`publint --strict`, then `scripts/check-package.mjs`, which fails on `dist/host`, an unresolved `@/` alias or a bare import that is not a declared dependency or peer) and `check:host`. `just ui-check` adds a frozen install and the route-tree diff, and CI runs it.
