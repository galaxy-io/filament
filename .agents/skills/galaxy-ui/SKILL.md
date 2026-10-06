---
name: galaxy-ui
description: Design and build Galaxy product UI (Filament, the GX app, any Galaxy React surface) with the @galaxy-io/dls component library. Use whenever someone is designing or implementing a Galaxy screen, page, form, table, dashboard, dialog, settings page or empty state; choosing between DLS components (Popover vs Dropdown vs Menu, SelectInput vs ToggleInput, Alert vs Toast, which table or chart); styling with DLS tokens (t.color, t.space, Linaria); reviewing Galaxy UI for brand, density, accessibility or dark/light parity; or writing code that imports from @galaxy-io/dls.
---

# Building Galaxy UI with @galaxy-io/dls

Galaxy UI is **quiet, dense and monochrome**. Layout and type carry the hierarchy; color carries status and category only; borders are 0.5px hairlines; every control sits on one of five rungs; every screen works in Dark and Light. `@galaxy-io/dls` (2.0) implements all of it: build screens from its components and tokens, and add as little of your own styling as possible.

## Workflow

1. **Understand the screen.** What is the one job? What data does it show, from where? What is the primary action (one per view)? What are the empty, loading, error and permission states? Is it a page, a panel beside a list, or a blocking task?
2. **Pick components** with the decision tables in [references/components.md](references/components.md). Prefer the component that already has the behavior (keyboard, focus, loading, empty) over composing your own.
3. **Compose with layout.** `Flex` and `Grid` for arrangement with `gap` / `padding` from the space scale; `Box` for surfaces, sizing and scrolling regions; `Widget` for every card. Start from a recipe in [references/recipes.md](references/recipes.md) when one fits.
4. **Apply tokens** only where components do not cover the need: Linaria `styled` / `css` reading `t` ([references/tokens.md](references/tokens.md)). Never hex, never margins.
5. **Check** states, accessibility and both themes with the checklist below, and the UX guidance in [references/patterns.md](references/patterns.md).

## Setup (once per app)

```tsx
// main.tsx
import "@galaxy-io/dls/styles.css";
import "@galaxy-io/dls/tokens.css";
import "@galaxy-io/dls/fonts.css";

import type { ReactNode } from "react";
import { createRoot } from "react-dom/client";
import GalaxyProvider from "@galaxy-io/dls/theme/GalaxyProvider";

declare function App(): ReactNode;

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <GalaxyProvider>
      <App />
    </GalaxyProvider>,
  );
}
```

- `GalaxyProvider` is the only provider: theme (`GalaxyTheme.SYSTEM` default, `DARK`, `LIGHT`), the overlay stack, toasts. Never mount `OverlayProvider` or `ToastProvider` yourself.
- Vite: `galaxyDls()` from `@galaxy-io/dls/vite` sets up Linaria for app styles. Biome: `"extends": ["@galaxy-io/dls/biome"]`.
- Peers: `react`, `react-dom`. Optional peers per subpath: `d3-scale` + `d3-shape` for charts, `@tanstack/react-table` + `@tanstack/react-virtual` for tables (and select lists), `react-syntax-highlighter` for `CodeBlock`, `react-markdown` + `remark-gfm` + `rehype-raw` for `MarkdownText`, `cronstrue` for `CronInput`, `@uiw/react-color` for `ColorInput`, the CodeMirror family for `CodeEditor`. `framer-motion` installs with the package.

## Imports

Every module is a subpath; the default export is the component; its enums come from the same module. No barrel exists.

```tsx
import { PlusIcon } from "@phosphor-icons/react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Span from "@galaxy-io/dls/text/Span";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

export const PageHeader = () => (
  <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
    <Flex direction={FlexDirection.COLUMN} gap={4}>
      <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
        Sources
      </Text>
      <Text variant={TextVariant.SECONDARY}>
        12 connected · <Span family={FontFamily.MONO}>48,210</Span> rows
      </Text>
    </Flex>
    <Flex gap={8}>
      <Button label="Import" variant={ButtonVariant.SECONDARY} />
      <Button label="New source" icon={PlusIcon} />
    </Flex>
  </Flex>
);
```

Shared enums (`Placement`, `Orientation`, `Side`, `Radius`, `FontFamily`, `GalaxyTheme`, `ReducedMotion`) come from `@galaxy-io/dls/theme/enums`.

Icons are Phosphor **components** (`PlusIcon`, not `<PlusIcon />`) passed to `icon` props or to `Icon`.

## Brand rules

| Rule | Means |
|---|---|
| Monochrome | No accent, no brand blue. The primary button, checked controls, selection, focus, links and active nav are neutral (`solid.primary`, `background.selected`, `border.focused`, `text.primary`). |
| Hairlines | `0.5px` borders (`HAIRLINE_BORDER`). No shadows, gradients or glows. |
| Radius | 2px controls and inline marks, 4px panels, `pill`. Nothing rounder. |
| Type | ABC Diatype for UI, Berkeley Mono for data and ids, Cardo for editorial copy only. Two weights: 400 and 500. Default step `body_md` 13px. |
| Spacing | `gap` / `padding` from `0 2 4 8 12 16 24 32 48`. No margins anywhere. |
| Rungs | Controls are `X_SMALL 20 · SMALL 24 · MEDIUM 32 (default) · LARGE 36 · X_LARGE 40` px. One rung per row. |
| Color | Status color (`success`, `warning`, `error`) only for status, through `variant`. Categorical color (17 families) only for categories, through `color`. Never decoration, never color alone. |
| Motion | 75ms color, 150ms enter / exit / transforms, off under reduced motion. |
| Feel | Refined, never redesigned: dense, aligned to the pixel grid, quiet. |

## Rules that catch most mistakes

- **One `PRIMARY` button per view.** Others `SECONDARY`; low emphasis and icon-only `TERTIARY`; destructive `ERROR` (only for delete or irreversible).
- **Cards are `Widget`s**; `Box` is for layout. Never a bordered, rounded `Box` with a heading.
- **Text goes through `Text` and `Span`.** No raw `<p>`, `<h1>`, `<span>`; heading semantics via `Text as="h2"`.
- **Every field has a visible label** (`label` on the input, or a wrapping `Field`, never both). Placeholders are examples.
- **Components never validate.** Compute the message and pass `error`.
- **`onChange` gets the value**, not the event. Overlays use `isOpen` / `defaultIsOpen` / `onOpenChange`.
- **Icon-only controls need `ariaLabel`** (and a `tooltip` with the same words).
- **`variant` is meaning, `color` is category**; never both. `hex` only for colors that are data.
- **Toolbars and the `Topbar` use `SMALL`** controls; forms and dialogs `MEDIUM`.
- **Tables:** bound the parent height, pass `getRowId`, mark the naming column `isRowHeader`, right-align numbers.
- **Keep overlays mounted** and drive them with `isOpen`; confirm destructive actions with `ConfirmDialog`.
- **Copy** is sentence case, buttons are verbs ("Save connection"), no "please", errors say what to do.

## Quick decisions

| Need | Use |
|---|---|
| Row / column layout | `Flex` (`direction={FlexDirection.COLUMN}` for a column) |
| Card | `Widget` (`header`, `actions`, `footer`, `isFlush` for tables and charts) |
| Hint on hover | `Tooltip` · interactive anchored panel: `Popover` · panel with Apply: `Dropdown` · commands: `Menu` |
| Blocking task | `Modal` · yes/no before an action: `ConfirmDialog` · detail beside a list: `Drawer` · ⌘K: `CommandPalette` |
| One of 2–5 visible options | `ToggleInput` (modes) or `RadioGroup` (forms with descriptions) · one of many: `SelectInput` · several: `MultiSelectInput` / `CheckboxGroup` |
| On/off applied now | `SwitchInput` · confirmed on submit: `CheckboxInput` |
| Lasting problem on screen | `Alert` · result of an action: `useToast()` · live state of a row: `Beacon` · status value: `Chip` |
| Loading with known shape | `Skeleton` · unknown: `Spinner` · known total: `ProgressBar` / `ProgressCircle` |
| Nothing to show | `EmptyState` (header, description, one action) |
| Read-only table | `InfiniteTable` · huge lists: `VirtualizedInfiniteTable` · editable cells: `InfiniteSpreadsheet` · one record: `DescriptionList` |
| Trend over time | `LineChart` · parts over time: `AreaChart` · compare categories: `BarChart` · parts of a whole: `PieChart` · two dimensions: `Heatmap` · KPI: `StatChart` · inline trend: `Sparkline` |
| Hierarchy | `Tree` · JSON payload: `JsonViewer` |
| App frame | `SidebarNav` + `Topbar` in plain `Flex` / `Box` (see recipes) |

The full catalogue, with every module, props and an example each: [references/components.md](references/components.md).

## Styling your own pieces

```tsx
import { styled } from "@linaria/react";
import { FOCUS_RING, HAIRLINE_BORDER, TRUNCATE } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

export const Tile = styled.button`
  ${HAIRLINE_BORDER}
  ${FOCUS_RING}
  display: flex;
  flex-direction: column;
  gap: ${t.space[4]};
  padding: ${t.space[12]};
  border-radius: ${t.radius.lg};
  background: ${t.color.background.primary};
  color: ${t.color.text.primary};
  transition: background-color ${t.duration.fast};
  &:hover {
    background: ${t.color.background.hovered};
  }
`;

export const TileMeta = styled.span`
  ${TRUNCATE}
  color: ${t.color.text.tertiary};
  font-size: ${t.font.sans.size.body_sm};
  line-height: ${t.font.sans.height.body_sm};
`;
```

Roles: `t.color.background.{base primary secondary tertiary hovered pressed selected disabled}`, `t.color.text.{primary secondary tertiary disabled}`, `t.color.border.{primary secondary tertiary hovered focused disabled}`, `t.color.solid.<role>.{background text hovered pressed}`, plus `success warning error` and the 17 families on every property. Details: [references/tokens.md](references/tokens.md).

## Before you hand it over

- [ ] One primary action; everything else lower emphasis; destructive actions confirmed.
- [ ] Every state designed: empty (`EmptyState`), loading (`Skeleton` / `isLoading`), error (`Alert` or field `error`), disabled with a reason.
- [ ] Every control on a rung, one rung per row; spacing from the scale; no margins, hex or shadows.
- [ ] Every field labelled; every icon-only control has `ariaLabel` + `tooltip`; charts, tables and trees have `ariaLabel`.
- [ ] Keyboard: everything reachable; overlays close on Escape and return focus; shortcuts shown with `hotKeys`.
- [ ] Dark and Light both checked; nothing color-only; status color only for status.
- [ ] Copy: sentence case, verb buttons, specific errors.

## References

| File | Read when |
|---|---|
| [references/components.md](references/components.md) | Choosing a component or looking up its props; every module of the package, with decision tables. |
| [references/tokens.md](references/tokens.md) | Styling anything yourself; picking a surface, text, border or status role; chart colors; the inverse scope. |
| [references/patterns.md](references/patterns.md) | Laying out a page, a form, a table view, overlays, feedback, empty states, charts; copy and accessibility. |
| [references/recipes.md](references/recipes.md) | Starting a screen: settings page, table page with filters and row actions, form in a modal, dashboard, detail page with tabs, empty state, command palette. |
| [references/migration.md](references/migration.md) | Working in an app still on 1.x (`FlexWrapper`, `withTheme`, `GalaxyThemeProvider`). |

Source and full docs: https://github.com/galaxy-io/dls (Storybook: https://storybook.getgalaxy.io).
