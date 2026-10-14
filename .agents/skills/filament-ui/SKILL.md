---
name: filament-ui
description: Design and build Filament's web UI (ui/) the way the product and the house want it. Who uses Filament, how its core flows should feel, the voice, the interaction rules, and the concrete app conventions (the standalone and module builds, routes, URL state, the query layer, screens, forms, dashboards, gates). Use whenever someone designs, builds or reviews anything under ui/, adds a screen, route, search param, query hook, mutation, form, table, dashboard panel, drawer, modal or settings section, wires a new RPC into the UI, or writes Filament UI copy. Load the galaxy-ui skill alongside it for the @galaxy-io/dls components and tokens.
---

# Building Filament UI

This skill lets an engineer who does not usually write frontend ship a Filament screen that looks, reads and behaves as if the people who designed Filament built it. It covers the product (who uses it, how it should feel) and the app (where code goes, how it is shaped). The design system itself, `@galaxy-io/dls`, is covered by the **`galaxy-ui`** skill.

## Before you start

1. **Load the `galaxy-ui` skill.** It owns every DLS component and prop, the tokens, the DLS-wide UX patterns and complete DLS recipes. Use it to pick and compose components. Storybook is at [storybook.getgalaxy.io](https://storybook.getgalaxy.io).
2. **Read [product.md](./references/product.md).** Know which core flow your screen serves and how it should feel before you choose a component.
3. **Find the nearest existing screen** in [screens.md](./references/screens.md) and open it. New screens copy an existing shape.

**When the two skills disagree, this one wins for Filament.** galaxy-ui describes the generic Galaxy app that `create-galaxy-app` scaffolds. Filament is a module that a Galaxy host mounts, and it departs in these places.

| Topic | galaxy-ui says | Filament does |
|---|---|---|
| Frame | content in an island, 12px from the frame | flat. DLS `AppFrame` through `FilamentLayout`, no island, gutter, border or radius around content |
| App shape | `src/routes`, `src/api` transport, `src/mock` | `src/host/**` is standalone-only, `src/module/**` is the package surface, everything else is library code. No mock transport, verify against a real server |
| Reading the URL | `useSearch({ from })` | the `module/hooks.ts` read hooks, which parse with the zod schema. Library code never calls `useSearch`, `useParams` or `useNavigate` |
| Writing the URL | `navigate({ to: "." })` | `useFilamentSearchUpdate`, `useFilamentNavigate`, `FilamentPath`, `createFilamentHref`. Never an absolute path or a route id |
| int64 in the URL | a serialization patch in `main.tsx` | no patch. `z.coerce.bigint()` and the router writes primitives with `String` |
| One record | `DescriptionList` | `KeyValueList` + `KeyValueListRow` |
| Status marks | `Beacon` for the live state of a row | run status is always a square `PipelineRunStatusSwatch`. `Beacon` only for non-run live state |
| Components | arrow functions, named exports allowed | `const X: FC<XProps>`, one default export per file |
| Comments | none that restate the code | zero comments. Only `biome-ignore` |
| `styled` | two or three per file at most | no hard cap, but as few as possible. Props first, always |
| Gates | typecheck, lint, format, build | `pnpm check`, the [workflow.md](./references/workflow.md) greps and the route-tree diff |
| Icon-only tooltips | always | not on the canvas panel chrome (open panel, Back, Delete, Close, activity caret), `ariaLabel` only |

## The product in one glance

| | |
|---|---|
| Users | Engineers set Filament up. Ops and analytics people watch it run. Design for ops, hide nothing from engineers |
| Feel | **Fast, calm, safe, in that order.** The earlier one wins a trade-off |
| Picture | Vercel. Run history is its deployment list, run activity is its build log |
| Voice | Plain and technical. Name the thing, say what happened, stop. No exceptions |
| Size | Works from 1024px wide up. No mobile |
| Core flows | Connect a source or sink, create a pipeline, shape on the canvas, schedule and notify, run and watch, triage a failure, the daily glance |

## Workflow

1. **Understand the screen.** Which core flow, who is there, its one job, the entity it shows, the RPC that provides it, the primary action, the shell it matches. Its loading, empty, filtered-empty, error and permission states. Page or drawer ([product.md](./references/product.md#interaction-rules)).
2. **Wire the data.** Proto, `pnpm codegen`, `src/api/queries/<resource>.ts` ([data.md](./references/data.md)).
3. **Add the route** and its search params ([routing.md](./references/routing.md)).
4. **Compose the page** from the shell and DLS components in the house file shape ([screens.md](./references/screens.md), [architecture.md](./references/architecture.md)). Forms follow [forms.md](./references/forms.md), dashboards follow [dashboards.md](./references/dashboards.md). Add its keyboard shortcuts and command palette items ([product.md](./references/product.md#keyboard)).
5. **Run the gates, verify in the browser** and take the proof screenshots ([workflow.md](./references/workflow.md)).

## The stack in one glance

| | |
|---|---|
| Entry | `src/host/main.tsx` imports the three DLS stylesheets, mounts `GalaxyProvider`, `TransportQueryClientProvider` and the router |
| Module | `src/module/` is the package surface: `FilamentLayout`, `FilamentPath`, the `*RouteOptions`, the URL hooks and schemas, the command items |
| Routing | TanStack Router. Standalone file routes in `src/host/routes/` spread the module's route options, `src/host/routeTree.gen.ts` is committed |
| Data | ConnectRPC through `@connectrpc/connect-query`, one file per resource in `src/api/queries/`, proto messages from `src/gen/` |
| Shells | DLS `AppFrame` (through `FilamentLayout`), DLS `PageLayout` on every page, DLS `PendingLayout` / `EmptyLayout` / `ErrorLayout` |
| Styling | `Flex` / `Box` / `Grid` props first, then Linaria `styled` with `t` tokens through `galaxyDls({ prefix: "filament" })` |
| Gates | `pnpm check` from `ui/`, `just ui-check` from the root. No test runner |

## Imports

Every DLS module is a subpath whose default export is the component and whose enums are named exports from the same module. App code uses the `@/` alias. Biome orders the groups.

```tsx
import { type FC, useMemo } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import { formatRelativeTime } from "@galaxy-io/dls/utils/format";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/pipelines/PipelineName";

import { useFilamentFlowOpen, usePipelinesSearch } from "@/module/hooks";

import { createListPipelinesInput, useSuspenseListPipelinesInfiniteQuery } from "@/api/queries/pipelines";

import { formatTimestamp } from "@/utils/format";
```

## Rules that catch most mistakes

**Product**

- Know the flow. A screen that serves none of the seven core flows needs a reason.
- Fast before calm before safe. Confirm only deletes and anything that drops data or credentials, by typed name. Never confirm run, rename or save.
- Copy is plain and technical. No "Oops", "Please", "successfully" or exclamation marks, error pages included.
- Failures are stacked. A plain one-line summary, then the server's raw message in mono, always visible.
- Times are relative with an absolute tooltip in lists, absolute in detail views.
- A healthy screen is monochrome. Status color only on what is failing or live, category color only from the one registry map.
- Every new top-level page adds its command palette items. The primary create button binds `C`, list search binds `/`, section Save binds `⌘↵`.
- Bulk actions and row selection need a yes from a human first.

**Design system** (details in galaxy-ui)

- One `PRIMARY` button per view. Others `SECONDARY`, low emphasis and icon-only `TERTIARY`, `ERROR` only for delete or irreversible.
- Cards are `Widget`s. `Box` is for layout. Text goes through `Text` and `Span`, never raw `<p>` or `<span>`.
- Every field has a visible label. Placeholders are examples ending in three dots. Components never validate, pass `error` as `string | undefined`.
- Icon-only controls need `ariaLabel` and a `tooltip` with the same words. The canvas panel chrome is the one exception and carries `ariaLabel` only ([product.md](./references/product.md#interaction-rules)).
- `variant` is meaning, `color` is category, never both. No hex, margins or shadows.
- Pass `tooltip` on a `Chip`. Wrapping it in `Tooltip` turns it into a button.
- A missing DLS piece is composed from DLS parts with as little `styled` as possible, then requested as a GitHub issue on `galaxy-io/dls`. Never select a DLS internal class.

**App conventions**

- Every component is `const X: FC<XProps>`, props interfaces are not exported. One default export per file, filename equals the export, feature prefix on every name, per-folder `constants.ts` (maps) / `types.ts` / `utils.ts` (pure functions), no `index.ts`, zero comments.
- Types come from the protos. `Pipeline["id"]`, `Record<RunStatus, …>` exhaustive, `z.enum(ProtoEnum)` in URLs, `UNSPECIFIED` handled by behaviour.
- Maps over ternaries (`X_TO_Y_MAP`). `match(enum).with(…).exhaustive()` to branch per member.
- State is one `XState` object with a `DEFAULT_STATE` and `handleX` setters. No `useEffect` in `.tsx`, effects live in named `hooks/useX.ts`. Derived values are computed at render.
- Shared view state lives in the URL. Schemas in `module/schemas.ts` with every field `.optional().catch(undefined)`, read through the `module/hooks.ts` read hooks, written with `useFilamentSearchUpdate((prev) => ({ ...prev, key }))`, `replace: true` for view state.
- Library code never names an absolute route. Paths are `FilamentPath` members, hrefs come from `createFilamentHref`, navigation goes through `useFilamentNavigate`.
- Data goes through `src/api/queries`. Pages use suspense hooks, overlays and dashboards use plain hooks, loading gates on `isLoading`, invalidation lives in the mutation hook. Poll only while something is active.
- Components take ids and fetch their own entity. Prefer an entity Get RPC over list-find on the client.
- Whole-view loading is `PendingLayout`. Content-shaped loading is a `Skeleton` sized like the value. Errors are `ErrorLayout`, empty is `EmptyLayout`, never one standing in for another.
- Settings sections are collapsible `Widget`s with local state and one Save each. Only name and description edit inline.
- App-wide overlays are search params rendered by `FilamentLayout`. They stay mounted with `isOpen`, take their record as props, reset per open with `useOverlaySession`, and never read search params themselves.
- Shared components own their copy. Callers pass semantic booleans (`hasStoredSecret`), not strings.

## Quick decisions

These are Filament's choices. galaxy-ui's component tables cover the rest.

| Need | Use |
|---|---|
| Card or section | `Widget` (`header`, `actions`, `isCollapsible defaultIsOpen`, `isFlush` for tables and charts) |
| Page chrome | DLS `PageLayout` (`header`, `actions`, `banner`, `tabs`, `toolbar`) · panel or dropdown title row: `BaseHeader` (`components/BaseHeader.tsx`) |
| Top-level list of entities | `PageLayout` with a `ListSearch` toolbar + `InfiniteTable` or a `Grid` of cards paged by `useEndReached` |
| Detail of an entity | it has parts: a page with `tabs` · one record beside its list: a `Drawer` opened by a search param through `FilamentLayout` |
| Blocking task | `Modal` opened by a `Flow` param · multi-step: a provider folder + `Stepper` sidebar |
| Yes/no before an action | `useConfirm` + DLS `ConfirmDialog` (`confirmValue` + `isMatch` for a danger zone, an `Alert` as `children` for a caveat) |
| Lasting problem on screen | `Alert` (`variant` for status, `color` for a category such as the schedule banner) · result of an action: `useToast()` · status value: `Chip` · run status: `PipelineRunStatusSwatch` |
| Loading | whole view `PendingLayout` · known shape `Skeleton` in a sized `Box` · chart/table `isLoading` |
| Nothing to show | DLS `EmptyLayout` (`graphic` for a feature first-run graphic) · filtered: `EmptyLayout icon={MagnifyingGlassIcon}` · went wrong: DLS `ErrorLayout` |
| One record | `KeyValueList` + `KeyValueListRow` · JSON payload: `CodeBlock` · nested table: `InfiniteTable hasHeader={false}` |
| Trend over time | `LineChart` · stacked series: `AreaChart` · counts per bucket: `BarChart` · KPIs: `BigNumberGroup` of `BigNumber`s · linked charts: `ChartGroupProvider` |
| Panes beside a canvas or list | `ResizablePanels hasDividers storageKey`, the main pane fluid with a `min`. Collapse is a conditional panel plus a header outside the group |
| A scrolling region | `FlexItem grow={1} minHeight={0}` › `ScrollArea` › `Box padding`, never `overflow: auto` (the DLS reset hides native scrollbars) · a bounded table never scrolls the page: `Flex direction={COLUMN} grow={1} basis={0} minHeight={0}` |
| A time | lists: `formatRelativeTime` with `formatTimestamp` in the tooltip · detail views: `formatTimestamp` |
| Any other value | DLS `utils/format` (`formatNumber`, `formatBytes`, `formatDuration`, all `bigint`-safe) · versions: `formatVersion` · entity names: the domain's `formatX` |

## Styling your own pieces

Reach for `Flex`, `Box`, `Grid` and `FlexItem` props first. A `styled` component is only for what props cannot express (a fixed frame, positioning, a hand-made button, xyflow chrome). There is no hard cap per file, but every one needs a reason, and fewer is always better. Name it for its role, keep it unexported, use tokens only.

```tsx
import { styled } from "@linaria/react";

import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const PipelinePageSaveIssueRow = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  width: 100%;
  padding: ${t.space[8]};
  display: flex;
  align-items: center;
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  background-color: ${t.color.background.primary};
  transition: background-color ${t.duration.fast};

  &:hover:not(:disabled) {
    background-color: ${t.color.background.hovered};
  }
`;
```

The build prefixes every class with `filament-`, which keeps the module's stylesheet out of a host's way. Token roles are in galaxy-ui, app habits in [architecture.md](./references/architecture.md#styling-inside-the-app).

## Before you hand it over

- [ ] The screen serves a named core flow and feels the way [product.md](./references/product.md) says it should.
- [ ] One primary action. Only deletes and data or credential loss are confirmed, by name.
- [ ] Loading, empty, filtered-empty, error and permission states designed, and all different. Failures stacked, summary over raw.
- [ ] Copy is plain, sentence case, verb buttons, specific errors. Times relative with an absolute tooltip in lists.
- [ ] Every field labelled, every icon-only control has `ariaLabel` + `tooltip` (except the canvas panel chrome), charts and tables have `ariaLabel`.
- [ ] Keyboard. `C`, `/` and `⌘↵` bound where they apply, rows open with `↵`, overlays close on Escape and restore focus, palette items added.
- [ ] Files, names, state, URL params and queries follow the app conventions. `pnpm check` passes, the [workflow.md](./references/workflow.md) greps print nothing, `src/host/routeTree.gen.ts` is committed.
- [ ] Screenshots of the changed screen in Dark, in Light and at 1024px wide.

## References

| File | Read when |
|---|---|
| [references/product.md](./references/product.md) | Always first. Users, how Filament should feel, voice, screen size, the seven core flows, the interaction rules, keyboard. |
| [references/architecture.md](./references/architecture.md) | Creating files. The two builds, the package surface, folders, naming, the file contract, imports, proto types, the state doctrine, styling habits, the recipe for building a Galaxy module. |
| [references/routing.md](./references/routing.md) | Adding a route or a search param, reading or writing URL state, route options, opening an overlay from anywhere, auth in the standalone. |
| [references/data.md](./references/data.md) | Wiring an RPC. The query file contract, building requests, which hook where, data ownership, mutations and toasts, cache gotchas. |
| [references/screens.md](./references/screens.md) | Starting a screen. The shells, list pages, tables, settings pages, drawers, modals, confirmations, the state layouts, status marks, formatting and copy. |
| [references/forms.md](./references/forms.md) | Building a form. Local-state forms, validation timing, schema-driven fields, the provider contract, wizards. |
| [references/dashboards.md](./references/dashboards.md) | Building a dashboard or a chart. URL view state, loading per panel, DLS charts, KPI tiles, filters and drill-down. |
| [references/workflow.md](./references/workflow.md) | Shipping. Proto to verified screen step by step, the gates, browser verification and proof, symptoms that are not bugs, the final checklist. |
| the `galaxy-ui` skill | Choosing a DLS component or looking up its props, tokens, DLS-wide UX patterns, DLS-only recipes. |
