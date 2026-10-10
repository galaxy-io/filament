---
name: filament-ui
description: Design and build the Filament web UI (ui/) and any Galaxy React surface with the @galaxy-io/dls component library and Filament's app conventions. Use whenever someone is designing or implementing a Galaxy screen, page, list, form, table, dashboard, drawer, modal, settings page or empty state; adding a route, a search param, a query hook or a mutation in ui/; wiring a new RPC into the UI; choosing between DLS components (Popover vs Dropdown vs Menu, SelectInput vs ToggleInput, Alert vs Toast, which table or chart); styling with DLS tokens (t.color, t.space, Linaria); reviewing Galaxy UI for brand, density, accessibility, dark/light parity or house conventions; or writing code that imports from @galaxy-io/dls.
---

# Building Galaxy UI

Galaxy UI is **quiet, dense and monochrome**. Layout and type carry the hierarchy, color carries status and category only, borders are hairlines, every control sits on one of five rungs, every screen works in Dark and Light. `@galaxy-io/dls` implements the look. Filament's `ui/` implements the way a Galaxy app is put together. This skill covers both, so a backend engineer can ship a complete screen without a designer or a frontend engineer in the loop.

Two layers of references live in this folder.

| Layer | Files | Source of truth |
|---|---|---|
| The design system | `components.md`, `tokens.md`, `patterns.md`, `recipes.md` | `@galaxy-io/dls`, updated with a DLS version bump |
| The app | `architecture.md`, `routing.md`, `data.md`, `screens.md`, `forms.md`, `dashboards.md`, `workflow.md` | Filament's `ui/src`, maintained here |

When the two disagree, the app layer wins for Filament. The known case is run status marks, which are squares here and not `Beacon`s.

Filament ships two ways from one `ui/src` tree. The standalone app (`src/host/**`) is embedded by the Go server. The module (`@galaxy-io/filament`) is mounted by a Galaxy host under any path. Library code never imports `src/host` and never names an absolute route. [architecture.md](./references/architecture.md) ends with the recipe for building any Galaxy app this way.

## Workflow

1. **Understand the screen.** What is its one job, what entity does it show, which RPC provides it, what is the primary action, and which existing shell does it match ([screens.md](./references/screens.md))? What are its empty, loading, filtered-empty, error and permission states?
2. **Wire the data.** Regenerate protos, add or extend `src/api/queries/<resource>.ts` ([data.md](./references/data.md)).
3. **Add the route** and its search params ([routing.md](./references/routing.md)).
4. **Compose the page** from the shell, DLS components and tokens ([components.md](./references/components.md), [tokens.md](./references/tokens.md), [patterns.md](./references/patterns.md)), in the house file shape ([architecture.md](./references/architecture.md)). Forms follow [forms.md](./references/forms.md), dashboards follow [dashboards.md](./references/dashboards.md).
5. **Run the gates and verify in the browser** in both themes ([workflow.md](./references/workflow.md)).

## The stack in one glance

| | |
|---|---|
| Entry | `src/host/main.tsx` imports the three DLS stylesheets, mounts `GalaxyProvider`, `TransportQueryClientProvider` and the router |
| Module | `src/module/` is the package surface: `FilamentLayout`, `FilamentPath`, the `*RouteOptions`, the URL hooks and schemas, the command items |
| Routing | TanStack Router. Standalone file routes in `src/host/routes/` spread the module's route options, `src/host/routeTree.gen.ts` is committed |
| Data | ConnectRPC through `@connectrpc/connect-query`, one file per resource in `src/api/queries/`, proto messages from `src/gen/` |
| Shells | DLS `AppFrame` (through `FilamentLayout`), DLS `PageLayout` on every page, DLS `PendingLayout` / `EmptyLayout` / `ErrorLayout` |
| Styling | Linaria `styled` with `t` tokens through `galaxyDls({ prefix: "filament" })`, `Flex` / `Box` / `Grid` props first, Biome extending `@galaxy-io/dls/biome` |
| Gates | `pnpm check` from `ui/`, `just ui-check` from the root |

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

Shared enums (`Placement`, `Orientation`, `Side`, `Radius`, `FontFamily`, `GalaxyTheme`) come from `@galaxy-io/dls/theme/enums`. Icons are Phosphor components with the `Icon` suffix (`PlusIcon`), passed to `icon` props or to `Icon component={…}`.

## Brand rules

| Rule | Means |
|---|---|
| Monochrome | No accent, no brand blue. The primary button, checked controls, selection, focus, links and active nav are neutral (`solid.primary`, `background.selected`, `border.focused`, `text.primary`). |
| Hairlines | 1px borders (`HAIRLINE_BORDER`, `HAIRLINE_WIDTH`). No shadows, gradients or glows. |
| Radius | 2px controls and inline marks, 4px panels (`t.radius.lg`), `pill`. Nothing rounder. |
| Type | ABC Diatype for UI, Berkeley Mono for data and ids, Cardo for editorial copy only. Two weights, 400 and 500. Default step `body_md` 13px, `body_sm` inside tables and panels. |
| Spacing | `gap` / `padding` from `0 2 4 8 12 16 24 32 48`. No margins anywhere. |
| Rungs | Controls are `X_SMALL 20 · SMALL 24 · MEDIUM 32 (default) · LARGE 36 · X_LARGE 40` px. One rung per row. `SMALL` in toolbars and navbars, `MEDIUM` in forms and dialogs. |
| Color | Status color (`success`, `warning`, `error`) only for status, through `variant`. Categorical color (17 families) only for categories, through `color`. Never decoration, never color alone. |
| Motion | 75ms color, 150ms enter / exit / transforms, off under reduced motion. Ambient motion is structural, never reactive to the pointer. |
| The frame | Flat. DLS `AppFrame` draws the sidebar column and a divider, `PageLayout` draws the page header. No island, gutter, border or radius around content. |
| Feel | Refined, never redesigned. Dense, aligned to the pixel grid, quiet. |

## Rules that catch most mistakes

**Design system**

- One `PRIMARY` button per view. Others `SECONDARY`, low emphasis and icon-only `TERTIARY`, destructive `ERROR` only for delete or irreversible.
- Cards are `Widget`s. `Box` is for layout. Any titled or bordered unit of content is a Widget.
- Text goes through `Text` and `Span`. No raw `<p>`, `<h1>`, `<span>`. Heading semantics via `Text as="h2"`.
- Every field has a visible label (`label` on the input, or a wrapping `Field`, never both). Placeholders are examples and end with three dots.
- Components never validate. Compute the message and pass `error` as `string | undefined`.
- `onChange` gets the value, not the event. Overlays use `isOpen` / `onOpenChange`.
- Icon-only controls need `ariaLabel` and a `tooltip` with the same words.
- `variant` is meaning, `color` is category, never both. `hex` only for colors that are data.
- Tables bound the parent height, pass `getRowId`, mark the naming column `isRowHeader`, right-align numbers in mono.
- Keep overlays mounted and drive them with `isOpen`. Confirm destructive actions with the entity's name.
- Pass `tooltip` on a `Chip`. Wrapping it in `Tooltip` turns it into a button.

**App conventions**

- Every component is `const X: FC<XProps>`, props interfaces are not exported. One default export per file, filename equals the export, feature prefix on every name, per-folder `constants.ts` (maps) / `types.ts` / `utils.ts` (pure functions), no `index.ts`, zero comments.
- Types come from the protos. `Pipeline["id"]`, `Record<RunStatus, …>` exhaustive, `z.enum(ProtoEnum)` in URLs, `UNSPECIFIED` handled by behaviour.
- Maps over ternaries and lookups (`X_TO_Y_MAP`). `match(enum).with(…).exhaustive()` to branch per member.
- State is one `XState` object with a `DEFAULT_STATE` and `handleX` setters. No `useEffect` in `.tsx`, effects live in named `hooks/useX.ts`. Derived values are computed at render.
- Shared view state lives in the URL. Schemas in `module/schemas.ts` with every field `.optional().catch(undefined)`, read through the `module/hooks.ts` read hooks (which parse, so nothing is cast), written with `useFilamentSearchUpdate((prev) => ({ ...prev, key }))`, `replace: true` for view state.
- Library code never names an absolute route. Paths are `FilamentPath` members, hrefs come from `createFilamentHref`, navigation goes through `useFilamentNavigate`.
- Data goes through `src/api/queries`. Pages use suspense hooks, overlays and dashboards use plain hooks, loading gates on `isLoading`, invalidation lives in the mutation hook.
- Components take ids and fetch their own entity. Prefer an entity Get RPC over list-find on the client.
- Whole-view loading is `PendingLayout`. Content-shaped loading is a `Skeleton` sized like the value. Errors are `ErrorLayout`, empty is `EmptyLayout`, never one standing in for another.
- Run status marks are squares from `PIPELINE_RUN_STATUS_TO_HUE_MAP`. `Beacon` is only for non-run live state.
- Settings sections are collapsible `Widget`s with local state and one Save each. No immediate-action RPCs on a settings page.
- App-wide overlays are search params rendered by `FilamentLayout`. They stay mounted with `isOpen`, take their record as props, reset per open with `useOverlaySession`, and never read search params themselves.
- Shared components own their copy. Callers pass semantic booleans (`hasStoredSecret`), not strings.
- Copy is sentence case, buttons are verbs ("Save connection"), toasts are "Noun past-tense" or "Verb failed", missing values are DLS `EMPTY_VALUE`.

## Quick decisions

| Need | Use |
|---|---|
| Row / column layout | `Flex` (`direction={FlexDirection.COLUMN}`), `Grid` for a card grid |
| Card or section | `Widget` (`header`, `actions`, `isCollapsible defaultIsOpen`, `isFlush` for tables and charts) |
| Page chrome | DLS `PageLayout` (`header`, `actions`, `banner`, `tabs`, `toolbar`) · panel or dropdown title row: `BaseHeader` (`components/BaseHeader.tsx`) |
| Top-level list of entities | `PageLayout` with a `ListSearch` toolbar + `InfiniteTable` or a `Grid` of cards paged by `useEndReached` |
| Detail of an entity beside its list | `Drawer` opened by a search param through `FilamentLayout` |
| Blocking task | `Modal` opened by a `Flow` param · multi-step: a provider folder + `Stepper` sidebar |
| Yes/no before an action | `useConfirm` + DLS `ConfirmDialog` (`confirmValue` + `isMatch` for a danger zone, an `Alert` as `children` for a caveat) · create and info dialogs: plain `Modal` |
| Hint on hover | `Tooltip` · interactive anchored panel: `Popover` · panel with Apply: `Dropdown` · commands: `Menu` |
| One of 2–5 visible options | `ToggleInput` · one of many: `SelectInput` · several: `MultiSelectInput` (pinned "All" through `selectAllLabel`) |
| On/off applied now | `SwitchInput` · confirmed on Save: `CheckboxInput` |
| Lasting problem on screen | `Alert` (`variant` for status, `color` for a category such as the schedule banner) · result of an action: `useToast()` · status value: `Chip` · run status: `PipelineRunStatusSwatch` |
| Loading | whole view `PendingLayout` · known shape `Skeleton` in a sized `Box` · chart/table `isLoading` |
| Nothing to show | DLS `EmptyLayout` (`graphic` for a feature first-run graphic) · filtered: `EmptyLayout icon={MagnifyingGlassIcon}` · went wrong: DLS `ErrorLayout` |
| Read-only table | `InfiniteTable` (`hasHeader={false}` for a nested one) · one record: `KeyValueList` + `KeyValueListRow` (preferred over DLS `DescriptionList`) · JSON payload: `CodeBlock` |
| Trend over time | `LineChart` · stacked series: `AreaChart` · counts per bucket: `BarChart` · KPIs: `BigNumberGroup` of `BigNumber`s · linked charts: `ChartGroupProvider` |
| A scrolling region | `FlexItem grow={1} minHeight={0}` › `ScrollArea` › `Box padding`; never `overflow: auto` (the DLS reset hides native scrollbars) · a bounded table never scrolls the page: `Flex direction={COLUMN} grow={1} basis={0} minHeight={0}` |
| Format a value | DLS `utils/format` (`formatNumber`, `formatBytes`, `formatDuration`, `formatRelativeTime`, `formatDate`, all `bigint`-safe) · proto timestamps and versions: `utils/format.ts` (`formatTimestamp`, `formatVersion`) · entity names: the domain's `formatX` |

The full catalogue with every module and its props is [components.md](./references/components.md).

## Styling your own pieces

Reach for `Flex`, `Box`, `Grid` and `FlexItem` props first. A `styled` component is for what props cannot express (a fixed frame, positioning, a hand-made button, xyflow chrome), named for its role, unexported, three per file at most. Templates use tokens, never raw pixels, and never select a DLS internal class.

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

The build prefixes every class with `filament-`, which is how the module's stylesheet stays out of a host's way.

Roles are `t.color.background.{base primary secondary tertiary hovered pressed selected disabled}`, `t.color.text.{primary secondary tertiary disabled}`, `t.color.border.{primary secondary tertiary hovered focused disabled}`, `t.color.solid.<role>.{background text hovered pressed}`, plus `success warning error` and the 17 families on every property. `$`-props only for continuous values. Details in [tokens.md](./references/tokens.md), app habits in [architecture.md](./references/architecture.md).

## Before you hand it over

- [ ] One primary action, destructive actions confirmed by name, every other action lower emphasis.
- [ ] Loading, empty, filtered-empty, error and permission states designed, and all different.
- [ ] Every control on a rung, one rung per row, spacing from the scale, no margins, hex or shadows.
- [ ] Every field labelled, every icon-only control has `ariaLabel` + `tooltip`, charts and tables have `ariaLabel`.
- [ ] Keyboard reachable, overlays close on Escape and restore focus.
- [ ] Dark and Light both checked, nothing color-only, status color only for status, run statuses are squares.
- [ ] Copy is sentence case with verb buttons and specific errors.
- [ ] Files, names, state, URL params and queries follow the app conventions. `pnpm check` passes and the [workflow.md](./references/workflow.md) greps print nothing.

## References

| File | Read when |
|---|---|
| [references/architecture.md](./references/architecture.md) | Creating files. The two builds, the package surface, folders, naming, the file contract, imports, proto types, the state doctrine, styling habits, and the recipe for building a Galaxy module. |
| [references/routing.md](./references/routing.md) | Adding a route or a search param, reading or writing URL state, route options, opening an overlay from anywhere, auth in the standalone. |
| [references/data.md](./references/data.md) | Wiring an RPC. The query file contract, building requests, which hook where, data ownership, mutations and toasts, cache gotchas. |
| [references/screens.md](./references/screens.md) | Starting a screen. The shells, list pages, tables, settings pages, drawers, modals, confirmations, the state layouts, status marks, formatting and copy. |
| [references/forms.md](./references/forms.md) | Building a form. Local-state forms, validation timing, schema-driven fields, the provider contract, wizards. |
| [references/dashboards.md](./references/dashboards.md) | Building a dashboard or a chart. URL view state, loading per panel, DLS charts, KPI tiles, filters and drill-down. |
| [references/workflow.md](./references/workflow.md) | Shipping. Proto to verified screen step by step, the gates, browser verification, symptoms that are not bugs, the final checklist. |
| [references/components.md](./references/components.md) | Choosing a DLS component or looking up its props. Every module of the package with decision tables. |
| [references/tokens.md](./references/tokens.md) | Styling anything yourself. Surface, text, border and status roles, chart colors, the inverse scope. |
| [references/patterns.md](./references/patterns.md) | DLS-wide UX guidance. Page structure, density, forms, tables, overlays, feedback, charts, writing, accessibility. |
| [references/recipes.md](./references/recipes.md) | Complete DLS-only screens that type-check against the package. |

Design system: `@galaxy-io/dls`, with Storybook at [storybook.getgalaxy.io](https://storybook.getgalaxy.io). App source: `ui/src` in this repository.
