---
name: filament-ui
description: Design and build the Filament web UI (ui/) and any Galaxy React surface with the @galaxy-io/dls component library and Filament's app conventions. Use whenever someone is designing or implementing a Galaxy screen, page, list, form, table, dashboard, drawer, modal, settings page or empty state; adding a route, a search param, a query hook or a mutation in ui/; wiring a new RPC into the UI; choosing between DLS components (Popover vs Dropdown vs Menu, SelectInput vs ToggleInput, Alert vs Toast, which table or chart); styling with DLS tokens (t.color, t.space, Linaria); reviewing Galaxy UI for brand, density, accessibility, dark/light parity or house conventions; or writing code that imports from @galaxy-io/dls.
---

# Building Galaxy UI

Galaxy UI is **quiet, dense and monochrome**. Layout and type carry the hierarchy, color carries status and category only, borders are hairlines, every control sits on one of five rungs, every screen works in Dark and Light. `@galaxy-io/dls` implements the look. Filament's `ui/` implements the way a Galaxy app is put together. This skill covers both, so a backend engineer can ship a complete screen without a designer or a frontend engineer in the loop.

Two layers of references live in this folder.

| Layer | Files | Source of truth |
|---|---|---|
| The design system | `components.md`, `tokens.md`, `patterns.md`, `recipes.md` | generated from `~/git/dls/docs`, copied here after a DLS release |
| The app | `architecture.md`, `routing.md`, `data.md`, `screens.md`, `forms.md`, `dashboards.md`, `workflow.md` | Filament's `ui/src`, maintained here |

When the two disagree, the app layer wins for Filament. The known case is run status marks, which are squares here and not `Beacon`s.

## Workflow

1. **Understand the screen.** What is its one job, what entity does it show, which RPC provides it, what is the primary action, and which existing shell does it match ([screens.md](./references/screens.md))? What are its empty, loading, filtered-empty, error and permission states?
2. **Wire the data.** Regenerate protos, add or extend `src/api/queries/<resource>.ts` ([data.md](./references/data.md)).
3. **Add the route** and its search params ([routing.md](./references/routing.md)).
4. **Compose the page** from the shell, DLS components and tokens ([components.md](./references/components.md), [tokens.md](./references/tokens.md), [patterns.md](./references/patterns.md)), in the house file shape ([architecture.md](./references/architecture.md)). Forms follow [forms.md](./references/forms.md), dashboards follow [dashboards.md](./references/dashboards.md).
5. **Run the gates and verify in the browser** in both themes ([workflow.md](./references/workflow.md)).

## The stack in one glance

| | |
|---|---|
| Entry | `main.tsx` imports the three DLS stylesheets, mounts `GalaxyProvider` then `TransportQueryClientProvider` then `<App />` |
| Routing | TanStack Router, file routes in `src/routes/`, `routeTree.gen.ts` committed, zod `validateSearch` on every route that reads a param |
| Data | ConnectRPC through `@connectrpc/connect-query`, one wrapper file per resource in `src/api/queries/`, proto messages from `src/gen/` |
| Shells | `MainLayout` + `MainLayoutListPage`, `PipelineLayout`, `AppLayout` for URL-driven overlays, `PendingLayout` / `EmptyLayout` / `ErrorLayout` |
| Styling | Linaria `styled` with `t` tokens, `Flex` / `Box` / `Grid` props first, Biome extending `@galaxy-io/dls/biome` |
| Gates | `pnpm typecheck`, `pnpm lint:check`, `pnpm format:check`, `pnpm build` from `ui/` |

## Imports

Every DLS module is a subpath whose default export is the component and whose enums are named exports from the same module. App code uses the `@/` alias. Biome orders the groups.

```tsx
import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { PlusIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { formatTimeAgo } from "@/utils/format";
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
| The island | Pages sit on `background.base`. Content lives in a `background.primary` island with a hairline border and `t.radius.lg`. |
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

- One default export per file, filename equals the export, feature prefix on every name, per-folder `constants.ts` / `types.ts` / `utils.ts`, no `index.ts`, no comments.
- Types come from the protos. `Pipeline["id"]`, `Record<RunStatus, …>` exhaustive, `z.enum(ProtoEnum)` in URLs, `UNSPECIFIED` handled by behaviour.
- Maps over ternaries and lookups (`X_TO_Y_MAP`). `match(enum).with(…).exhaustive()` to branch per member.
- State is one `XState` object with a `DEFAULT_STATE` and `handleX` setters. No `useEffect` in components. Derived values are computed at render.
- Shared view state lives in the URL. Every param `.optional().catch(undefined)`, read with `useSearch({ from })`, written with `navigate({ to: ".", search: (prev) => ({ ...prev, key }) })`, `replace: true` for view state.
- Data goes through `src/api/queries`. Pages use suspense hooks, overlays and dashboards use plain hooks, loading gates on `isLoading`, invalidation lives in the mutation hook.
- Components take ids and fetch their own entity. Prefer an entity Get RPC over list-find on the client.
- Whole-view loading is `PendingLayout`. Content-shaped loading is a `Skeleton` sized like the value. Errors are `ErrorLayout`, empty is `EmptyLayout`, never one standing in for another.
- Run status marks are squares from `PIPELINE_RUN_STATUS_TO_HUE_MAP`. `Beacon` is only for non-run live state.
- Settings sections are collapsible `Widget`s with local state and one Save each. No immediate-action RPCs on a settings page.
- App-wide overlays are `Flow` search params rendered by `AppLayout`. Overlay content never reads search params itself.
- Shared components own their copy. Callers pass semantic booleans (`hasStoredSecret`), not strings.
- Copy is sentence case, buttons are verbs ("Save connection"), toasts are "Noun past-tense" or "Verb failed", missing values are `—`.

## Quick decisions

| Need | Use |
|---|---|
| Row / column layout | `Flex` (`direction={FlexDirection.COLUMN}`), `Grid` for a card grid |
| Card or section | `Widget` (`header`, `actions`, `isCollapsible defaultIsOpen`, `isFlush` for tables and charts) |
| Page or panel title row | `BaseHeader` (`layouts/components/BaseHeader.tsx`), toolbar `BaseToolbar` |
| Top-level list of entities | `MainLayoutListPage` + `InfiniteTable` or a `Grid` of cards |
| Detail of an entity beside its list | `Drawer` opened by a search param through `AppLayout` |
| Blocking task | `Modal` opened by a `Flow` param · multi-step: a provider folder + `Stepper` sidebar |
| Yes/no before an action | `useConfirm` + DLS `ConfirmDialog` (`confirmValue` + `isMatch` for a danger zone, an `Alert` as `children` for a caveat) · create and info dialogs: plain `Modal` |
| Hint on hover | `Tooltip` · interactive anchored panel: `Popover` · panel with Apply: `Dropdown` · commands: `Menu` |
| One of 2–5 visible options | `ToggleInput` · one of many: `SelectInput` · several: `MultiSelectInput` (pinned "All" via `utils/select`) |
| On/off applied now | `SwitchInput` · confirmed on Save: `CheckboxInput` |
| Lasting problem on screen | `Alert` · result of an action: `useToast()` · status value: `Chip` · run status: `PipelineRunStatusSwatch` |
| Loading | whole view `PendingLayout` · known shape `Skeleton` in a sized `Box` · chart/table `isLoading` |
| Nothing to show | `EmptyLayout` (a centred DLS `EmptyState`; `graphic` for a feature `*EmptyGraphic` on a first-run page) · filtered: `EmptyLayout icon={MagnifyingGlassIcon}` |
| Read-only table | `InfiniteTable` · one record: `ConnectionDrawerList` + `ConnectionDrawerKeyValueRow` rows (Mitch prefers these over DLS `DescriptionList`) · JSON payload: `CodeBlock` |
| Trend over time | `LineChart` · counts per bucket: `BarChart` · KPI: `StatChart` (`trailing`, `isLoading`) · linked charts: `ChartGroupProvider` |
| A scrolling region | `FlexItem grow={1} minHeight={0}` › `ScrollArea` › `Box padding`; never `overflow: auto` (the DLS reset hides native scrollbars) · a bounded table never scrolls the page: `Flex direction={COLUMN} grow={1} basis={0} minHeight={0}` |
| Format a value | `utils/format` (`formatCount`, `formatTimeAgo`, `formatDuration`, `formatBytes`) · entity names: the feature's `formatX` |

The full catalogue with every module and its props is [components.md](./references/components.md).

## Styling your own pieces

Reach for `Flex`, `Box`, `Grid` and `FlexItem` props first. A `styled` component is a wrapper for what props cannot express (a fixed frame, a scroll region, absolute positioning, a hand-made button), named for what it wraps, two or three per file at most.

```tsx
import { styled } from "@linaria/react";
import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const ContentIsland = styled.div`
  flex: 1;
  min-height: 0;

  background-color: ${t.color.background.primary};

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};

  overflow: hidden;
`;

const SidebarButton = styled.button<{ $isActive?: boolean }>`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  width: ${PIPELINE_SIDEBAR_BUTTON_SIZE}px;
  height: ${PIPELINE_SIDEBAR_BUTTON_SIZE}px;
  border-radius: ${t.radius.md};
  background-color: ${({ $isActive }) => ($isActive ? t.color.background.selected : "transparent")};
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;
```

Roles are `t.color.background.{base primary secondary tertiary hovered pressed selected disabled}`, `t.color.text.{primary secondary tertiary disabled}`, `t.color.border.{primary secondary tertiary hovered focused disabled}`, `t.color.solid.<role>.{background text hovered pressed}`, plus `success warning error` and the 17 families on every property. `$`-props only for continuous values. Details in [tokens.md](./references/tokens.md), app habits in [architecture.md](./references/architecture.md).

## Before you hand it over

- [ ] One primary action, destructive actions confirmed by name, every other action lower emphasis.
- [ ] Loading, empty, filtered-empty, error and permission states designed, and all different.
- [ ] Every control on a rung, one rung per row, spacing from the scale, no margins, hex or shadows.
- [ ] Every field labelled, every icon-only control has `ariaLabel` + `tooltip`, charts and tables have `ariaLabel`.
- [ ] Keyboard reachable, overlays close on Escape and restore focus.
- [ ] Dark and Light both checked, nothing color-only, status color only for status, run statuses are squares.
- [ ] Copy is sentence case with verb buttons and specific errors.
- [ ] Files, names, state, URL params and queries follow the app conventions. `pnpm typecheck`, `pnpm lint:check`, `pnpm format:check`, `pnpm build` pass.

## References

| File | Read when |
|---|---|
| [references/architecture.md](./references/architecture.md) | Creating files. Folders, naming, the file contract, imports, proto types, the state doctrine, styling habits, minimalism. |
| [references/routing.md](./references/routing.md) | Adding a route or a search param, reading or writing URL state, opening an overlay from anywhere, auth in the tree. |
| [references/data.md](./references/data.md) | Wiring an RPC. The query file contract, building requests, which hook where, data ownership, mutations and toasts, cache gotchas. |
| [references/screens.md](./references/screens.md) | Starting a screen. The shells, list pages, tables, settings pages, drawers, modals, confirmations, the state layouts, status marks, formatting and copy. |
| [references/forms.md](./references/forms.md) | Building a form. Local-state forms, validation timing, schema-driven fields, the provider contract, wizards. |
| [references/dashboards.md](./references/dashboards.md) | Building a dashboard or a chart. URL view state, loading per panel, DLS charts, KPI tiles, filters and drill-down. |
| [references/workflow.md](./references/workflow.md) | Shipping. Proto to verified screen step by step, the gates, browser verification, symptoms that are not bugs, the final checklist. |
| [references/components.md](./references/components.md) | Choosing a DLS component or looking up its props. Every module of the package with decision tables. |
| [references/tokens.md](./references/tokens.md) | Styling anything yourself. Surface, text, border and status roles, chart colors, the inverse scope. |
| [references/patterns.md](./references/patterns.md) | DLS-wide UX guidance. Page structure, density, forms, tables, overlays, feedback, charts, writing, accessibility. |
| [references/recipes.md](./references/recipes.md) | Complete DLS-only screens that type-check against the package. |

Design system source: `~/git/dls` (Storybook with `pnpm storybook`). App source: `ui/src` in this repository.
