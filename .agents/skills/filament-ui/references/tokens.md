<!-- Generated from docs/theming.md by `pnpm skill:sync`. Edit that file, not this one. -->

# Theming and tokens

Every color, space, radius, duration, z-index and control size is a token in `src/theme/tokens/`. Components read tokens only as CSS custom properties, so a theme switch is one attribute change on `<html>` and nothing re-renders.

## Themes

| `GalaxyTheme` | Meaning |
|---|---|
| `SYSTEM` (default) | Follows `prefers-color-scheme` (or the provider's `detector`). |
| `DARK` | The dark theme. |
| `LIGHT` | The light theme. |

- Galaxy is **monochrome**. There is no accent and no custom theme: the primary action, checked controls, selection, focus, links, active navigation and the brand mark use neutral roles.
- `GalaxyProvider` writes `data-gx-theme="dark" | "light"` on `<html>` (or on a wrapper with `isScoped`). `tokens.css` defines every color and font variable under those selectors and the scales on `:root`; the package reset in `styles.css` applies under the same attribute.
- The provider writes the attribute in a layout effect. A server-rendered page emits `<html data-gx-theme="dark">` (or `"light"`) itself, so colors and the reset apply on the first paint instead of after hydration.
- `useGalaxyTheme()` gives `selectedTheme`, `activeTheme`, `setTheme` and `theme` (the resolved hex object, for canvas or chart code). `ThemeSwitcher` is the ready-made control.

## Reading tokens

| Where | How |
|---|---|
| Linaria (`styled`, `css`) | `t` from `@galaxy-io/dls/theme/tokens/t`: `${t.color.text.primary}`, `${t.space[12]}`, `${t.radius.md}`. Every leaf is a `var(--gx-…)` string. |
| Plain CSS | The variables: `var(--gx-color-text-primary)`, `var(--gx-space-12)`, `var(--gx-radius-md)`. The name is `--gx-` plus the token path in kebab case. |
| JS logic (canvas, a library that needs hex) | `useGalaxyTheme().theme.color.text.primary`, or `DARK_THEME` / `LIGHT_THEME` from `theme/tokens/dark` / `light`. Never for styling. |

```tsx
import { styled } from "@linaria/react";
import { FOCUS_RING, HAIRLINE_BORDER, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

export const Row = styled.button`
  ${INTERACTIVE_RESET}
  ${HAIRLINE_BORDER}
  ${FOCUS_RING}
  display: flex;
  gap: ${t.space[8]};
  padding: ${t.space[8]} ${t.space[12]};
  border-radius: ${t.radius.md};
  background: ${t.color.background.primary};
  color: ${t.color.text.primary};
  transition: background-color ${t.duration.fast};
  &:hover {
    background: ${t.color.background.hovered};
  }
  &:active {
    background: ${t.color.background.pressed};
  }
`;
```

Never write a hex literal outside `src/theme/tokens/` (Biome's `no-hex-literal` rule), and never a raw px value where a token exists.

## Color: property first

`t.color.<property>.<role>`. One value per role; the property says where the color goes.

### `background`: fills

| Role | Use |
|---|---|
| `base` | The canvas: the page behind everything. |
| `primary` | Panels and cards on the canvas (`Widget`, `Box variant={PRIMARY}`), field fills. |
| `secondary` | Raised surfaces: a toolbar, a nested panel, the default `Chip`, the `BASE` button fill. |
| `tertiary` | The most elevated step: inline code, `Kbd` caps. |
| `hovered` | A neutral row or control under the pointer. |
| `pressed` | A neutral row or control while pressed. |
| `selected` | The chosen item: a menu row, a tree row, the active nav item, a selected table row, the selected segment of a `ToggleInput`. `text.primary` at 12% over `base`. |
| `disabled` | Disabled controls. |
| `success` `warning` `error` | The soft tint of a status (an `Alert`, a status `Chip`). |
| `<family>` | The soft tint of a category (`background.purple`). |

Surfaces are strictly ordered `base < primary < secondary < tertiary` in both themes.

### `text`: text and icons

| Role | Use |
|---|---|
| `primary` | Primary copy, titles, values, active icons. Also links and the Tabs indicator. |
| `secondary` | Supporting copy, labels in a description list, inactive tab labels. |
| `tertiary` | Hints, placeholders, metadata, timestamps, section labels. |
| `disabled` | Disabled controls. |
| `success` `warning` `error` | Status text and icons. |
| `<family>` | Categorical text and icons; chart marks. |

Icons use the same roles as text (`IconVariant` mirrors `TextVariant`).

### `border`: hairlines and frames

| Role | Use |
|---|---|
| `primary` | The default hairline: cards, dividers, field frames at rest. |
| `secondary` | Control borders that need a little more weight. |
| `tertiary` | Emphasized dividers. |
| `hovered` | A field frame under the pointer. |
| `focused` | Focus: `FOCUS_RING` and `FIELD_FOCUS`. One neutral step past `hovered`. |
| `disabled` | Disabled controls. |
| `success` `warning` `error` | Status borders (≥ 3:1 on every surface). |
| `<family>` | Category borders. |

### `solid`: strong fills with their text

Each solid role is `{ background, text, hovered, pressed }`: the fill, the text and icons drawn on it, and the two interaction states.

| Role | Use |
|---|---|
| `solid.primary` | The monochrome fill (`text.primary` with `background.base` on it): the `PRIMARY` button, checked checkbox, radio and switch, the current Stepper marker, the selected date, the brand mark. |
| `solid.neutral` | The muted fill (`background.secondary` with `text.primary` on it): the `BASE` button. Hover and press are visible steps in both themes. |
| `solid.success` `solid.warning` `solid.error` | Strong status fills: the `ERROR` button, the `Beacon` dot. A status dot is a `solid.<role>.background`, never a `text` role: `text` is tuned for copy on the canvas and makes a dark, muddy fill in Light. A `Circle` or `Square` swatch is the exception: it keys chart marks, so it draws the same `text.<role>` the marks do. |
| `solid.<family>` | Strong category fills, and a categorical `Beacon` dot. |

### `opacity`: overlays

`opacity.bg4 … bg100` is the canvas color at N% (the Modal backdrop is `bg48`); `opacity.fg4 … fg100` is the text color at N% (quiet marks such as `GridBackground`). Steps: `4 8 12 16 24 32 48 64 80 100`.

### States at a glance

| State | Background | Border | Text |
|---|---|---|---|
| Rest | `primary` (fields), none (rows) | `primary` | `primary` / `secondary` |
| Hover | `hovered` | `hovered` | unchanged |
| Pressed | `pressed` | unchanged | unchanged |
| Selected | `selected` | `text.primary` hairline on cards and chips | `primary` |
| Focus | unchanged | `focused` (ring or inside line) | unchanged |
| Disabled | `disabled` | `disabled` | `disabled` |
| Invalid | unchanged | `error` in every state | `error` message |

## Statuses

`success`, `warning` and `error` exist on every property (`background.success`, `text.success`, `border.success`, `solid.success`) with values of their own: a clear green, a warm amber that never reads brown, a clear red. They are not copies of a palette family, and warning never looks like error.

- Use them only for status: a run failed, a sync is degraded, a save succeeded.
- There is no `info`. Neutral news uses the neutral roles (`AlertVariant.PRIMARY`, `ToastVariant.PRIMARY`).
- Components take a status through `variant` (`ChipVariant.ERROR`, `BeaconVariant.SUCCESS`, `TextVariant.WARNING`). Every status component draws the same `text.<status>`, tint and border; a solid indicator (the `Beacon` dot) draws `solid.<status>.background`.

## Categorical palette

17 families: `red orange amber yellow lime green emerald teal cyan sky blue indigo violet purple fuchsia pink rose`. No gray: neutrals are the neutral roles.

- Each family exists on every property (`background.purple`, `text.purple`, `border.purple`, `solid.purple`) and follows the theme.
- Components take a category through the `color` prop (`PaletteColor`): `Chip`, `Beacon`, `Circle`, `Square`, `Avatar`, `Sparkline`, `ProgressBar`, `ProgressCircle`, `Timeline` items. `color` and `variant` are mutually exclusive by type: `variant` is meaning, `color` is category.
- Use a family for something that *is* a category (a team, a source type, a series), never to decorate.

```tsx
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";

export const Tags = () => (
  <>
    <Chip label="Design" color="purple" />
    <Chip label="Failed" variant={ChipVariant.ERROR} hasDot />
  </>
);
```

### The mode-stable palette

`t.palette.<family>[100 | 300 | 500 | 700 | 900]` and `PALETTE` (`@galaxy-io/dls/theme/tokens/palette`) are the raw Tailwind v4 steps (50, 200, 500, 800, 950): the same hex in both themes. They exist for app-side chart and canvas code that needs a fixed hex in both themes; DLS charts draw their series with the theme `text.<family>` roles, and a lint rule (`dls/palette-scope`) keeps the palette out of components. Components and app UI read the theme roles.

### Chart colors

DLS charts color series through `ChartPalette` slots, which map to the theme's `text.<family>` roles:

- Automatic order: `PURPLE PINK BLUE TEAL LIME ORANGE YELLOW GREEN` (status slots never join it, so a ninth series never reads as an error).
- `SUCCESS WARNING ERROR` slots for series that mean a status; `PRIMARY SECONDARY TERTIARY` are `indigo`, `sky`, `violet` (charts have no gray).
- A series `hex` overrides the slot, for colors that are data. It does not follow the theme.

## The inverse scope

There are no inverse tokens. Content that sits on the opposite theme's surface (a dark banner in a light app) is a scoped provider set to the opposite theme, with the ordinary roles inside it. Overlays portal to `document.body` by default, which sits outside a scoped wrapper: a scoped island that opens a Modal, Popover or Menu passes a `portalContainer` inside the island (an unscoped provider on `<html>` needs nothing).

```tsx
import type { ReactNode } from "react";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Text from "@galaxy-io/dls/text/Text";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import GalaxyProvider from "@galaxy-io/dls/theme/GalaxyProvider";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

export function Inverse({ children }: { children: ReactNode }) {
  const { activeTheme } = useGalaxyTheme();
  const inverse = activeTheme === GalaxyTheme.LIGHT ? GalaxyTheme.DARK : GalaxyTheme.LIGHT;
  return (
    <GalaxyProvider key={inverse} theme={inverse} isScoped>
      <Box variant={BoxVariant.BASE} padding={[8, 12]}>
        {children}
      </Box>
    </GalaxyProvider>
  );
}

export const TrialBanner = () => (
  <Inverse>
    <Text>Your trial ends in 3 days.</Text>
  </Inverse>
);
```

- A scoped provider without `theme` inherits the theme around it instead of following the OS.
- The `Tooltip` bubble is always an inverse scope; `TooltipVariant` picks the fill inside it.

## Focus

- `FOCUS_RING` (from `@galaxy-io/dls/styles/mixins`): a 1px `border.focused` outline, offset 1px, on keyboard focus only. Buttons, checkboxes, tabs, rows, cards.
- `FIELD_FOCUS`: field frames (every input, CodeEditor) draw `border.focused` as a 1px line **inside** the frame on any focus, so nothing shifts. An invalid field keeps `border.error`.
- `border.focused` is a neutral gray one step stronger than `border.hovered` in both themes: rest → hover → focus reads as one ramp.

## Scales

| Token | Values |
|---|---|
| `t.space[n]` | `0 2 4 8 12 16 24 32 48` (px). The `Space` type; `gap` and `padding` only. |
| `t.radius` | `sm` 2px (inline marks), `md` 2px (controls, focus ring), `lg` 4px (panels), `pill`. |
| `t.duration` | `fast` 75ms (color, border), `base` 150ms (transforms, enter and exit). |
| `t.z` | `dropdown` 1000, `drawer` 1100, `modal` 1200, `toast` 1300, `tooltip` 1400. |
| `t.size.control` | `xSmall` 20, `small` 24, `medium` 32, `large` 36, `xLarge` 40 (px). |

### Type

| Step | Sans size | Line height (UI) | Mono size |
|---|---|---|---|
| `caption` | 10 | 12 | 9 |
| `body_sm` | 12 | 16 | 11 |
| `body_md` (default) | 13 | 16 | 12 |
| `body_lg` | 14 | 20 | 13 |
| `heading_sm` | 16 | 20 | 15 |
| `heading_md` | 20 | 24 | 18 |
| `heading_lg` | 24 | 28 | 22 |
| `display_sm` | 32 | 40 | 29 |
| `display_lg` | 40 | 48 | 37 |

- Weights: `t.font.<family>.weight.regular` (400) and `medium` (500). Nothing bolder.
- Mono shares the sans line height, so a mono value and a sans label share a baseline.
- Serif (Cardo) has editorial steps only (`body_lg` and up), with deeper line heights; prose mode (`Text isProse`, `MarkdownText`) uses line height 1.5.
- In code: `t.font.sans.size.body_md`, `t.font.sans.height.body_md`, `t.font.sans.spacing.body_md`, `t.font.mono.family`. Prefer `Text` and `Span` over styling type by hand.

## Contrast guarantees

`src/theme/tokens/tokens.test.ts` checks, in both themes:

- Surfaces are monotonic (`base < primary < secondary < tertiary`); text steps are ordered `primary > secondary > tertiary > disabled`.
- Borders step `primary < hovered < focused`, all neutral.
- `text.primary`, `text.secondary` and `text.tertiary` reach 4.5:1 on every surface.
- `text.<status>` and `text.<family>` reach 4.5:1 on every surface and on their own tint.
- Status borders reach 3:1 on every surface.
- `solid.<x>.text` reaches 4.5:1 on its fill at rest, hover and press.
- `background.selected` is `text.primary` at 12% over `background.base`; warning and error never look alike; orange, amber and yellow text never look alike.

A token change that breaks one of these fails `pnpm verify`.

## Dark and light parity

Both themes declare exactly the same variables (a test checks it). Design and review every screen in both; `Storybook`'s Theme toolbar switches System / Dark / Light for every story and docs page.
