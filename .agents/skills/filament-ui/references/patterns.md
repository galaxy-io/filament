# Patterns

How Galaxy product surfaces (Filament, the GX app, every Galaxy React app) are put together with the DLS. Grounded in the Storybook examples and Do / Don't stories; when a story and this page disagree, the story wins and this page gets fixed.

The short version: **quiet, dense, monochrome**. Layout and type carry the hierarchy; color carries status and category only; every control sits on a rung; every state is designed in Dark and Light.

## Page structure

There are two frames and they never mix. A Galaxy module inside the host is an `AppFrame` around a `SidebarNav` with no `header`, with a `PageLayout` for each page. A standalone app is plain layout: a row with the nav column, a vertical hairline, and a column with a `Topbar` over the one scrolling region.

```tsx
import { useState } from "react";
import { DatabaseIcon, GearIcon, HouseIcon, SidebarSimpleIcon } from "@phosphor-icons/react";
import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import SidebarNav, { NavItem } from "@galaxy-io/dls/navigation/SidebarNav";
import Topbar from "@galaxy-io/dls/navigation/Topbar";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";

export function StandaloneFrame() {
  const [isCollapsed, setIsCollapsed] = useState(false);
  return (
    <Flex height="100vh">
      <Box width={isCollapsed ? 48 : 240} height="100%">
        <SidebarNav
          isCollapsed={isCollapsed}
          header={<GalaxyLogomark ariaLabel="Galaxy" />}
          footer={
            <>
              <NavItem label="Settings" icon={GearIcon} href="/settings" />
              <Button
                icon={SidebarSimpleIcon}
                ariaLabel={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
                variant={ButtonVariant.TERTIARY}
                size={ButtonSize.SMALL}
                onClick={() => setIsCollapsed(!isCollapsed)}
              />
            </>
          }
        >
          <NavItem label="Overview" icon={HouseIcon} href="/" isActive />
          <NavItem label="Sources" icon={DatabaseIcon} href="/sources" />
        </SidebarNav>
      </Box>
      <Divider orientation={Orientation.VERTICAL} />
      <Flex direction={FlexDirection.COLUMN} grow={1} minWidth={0}>
        <Topbar
          actions={
            <>
              <SearchInput size={InputSize.SMALL} />
              <Avatar name="Ada Lovelace" size={AvatarSize.SMALL} />
            </>
          }
        >
          Overview
        </Topbar>
        <Flex as="main" direction={FlexDirection.COLUMN} grow={1} minHeight={0} overflow="auto">
          <Flex direction={FlexDirection.COLUMN} gap={16} padding={24}>
            <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
              Overview
            </Text>
          </Flex>
        </Flex>
      </Flex>
    </Flex>
  );
}
```

- `SidebarNav` holds top-level destinations only, active from the route. In-page views are `Tabs`.
- **Module pages** use `PageLayout`. It draws the header band with the title and actions, then `banner`, `tabs` and `toolbar`, then a body that never scrolls, so the page wraps its scrolling content in a `ScrollArea`.
- `Topbar` holds the page title or `Breadcrumbs` and app-wide controls (search, account) at `SMALL`. Record actions and view tabs belong in the page header, not the bar.
- One scrolling region per page (the column under the `Topbar`), so the bar never scrolls away. Give it `minHeight={0}`.
- **Navigation bars with tabs** (a product navbar with destinations in the middle): put `Tabs` straight into the 48px row, which stretches it, not in a column pinned to the bottom. The labels then center on the same line as the logo and buttons, and the indicator sits on the bar's bottom hairline; lay the bar's hairline over the row's bottom edge so it meets the tabs' own in one line.
- **Page header** (drawn by `PageLayout` in a module): an `h1` (`HEADING_MD`, medium), an optional secondary line, and the page's actions at the end of the same row (one `PRIMARY`, the rest `SECONDARY`). Then `Tabs` if the object has peer views.
- **Page padding** `24`; gaps between page sections `16` or `24`; inside a card `12` or `16`.
- A detail view beside a list is a `Drawer` (modal or docked with `isModal={false}`) or `ResizablePanels`, not a new page, when the list stays relevant.

## Density and sizing

| Context | Rung |
|---|---|
| Forms, dialogs, page actions, standalone inputs | `MEDIUM` (32px), the default |
| `Topbar`, toolbars, filter bars, `Widget` headers, `Alert` actions, table toolbars | `SMALL` (24px) |
| Inside table rows, chips, dense lists, inline row actions | `X_SMALL` (20px) or `SMALL` |
| Prominent standalone actions, onboarding | `LARGE` (36px) |
| Hero calls to action | `X_LARGE` (40px) |

- Everything in one row shares a rung: a `SMALL` `SearchInput` beside `SMALL` buttons, chips at the matching `ChipSize`.
- Tables: `MEDIUM` (48px rows) by default, `SMALL` (32px) for dense data, `LARGE` (52px) for rows with two lines or avatars.
- `Widget` `size` sets card density; use `SMALL` in dashboards with many cards.
- Don't mix rungs to create hierarchy; use type and variants.

## Forms

```tsx
import { useState } from "react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Field from "@galaxy-io/dls/inputs/Field";
import Fieldset from "@galaxy-io/dls/inputs/Fieldset";
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

function validateName(name: string): string | undefined {
  if (name.trim() === "") return "Enter a connection name.";
  if (name.trim().length > 100) return "Use 100 characters or fewer.";
  return undefined;
}

export function ConnectionForm({ onSave }: { onSave: (name: string, host: string) => void }) {
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [nameError, setNameError] = useState<string | undefined>(undefined);
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        const error = validateName(name);
        setNameError(error);
        if (error === undefined) onSave(name, host);
      }}
    >
      <Flex direction={FlexDirection.COLUMN} gap={16}>
        <Field label="Connection name" description="Shown in the sidebar and in alerts." isRequired {...(nameError ? { error: nameError } : {})}>
          <TextInput
            value={name}
            onChange={(next) => {
              setName(next);
              if (nameError) setNameError(validateName(next));
            }}
            fillWidth
          />
        </Field>
        <Fieldset label="Server">
          <TextInput label="Host" value={host} onChange={setHost} family={FontFamily.MONO} placeholder="db.internal" fillWidth />
          <NumberInput label="Port" defaultValue={5432} min={1} max={65535} />
        </Fieldset>
        <Flex justifyContent={JustifyContent.END} gap={8}>
          <Button label="Cancel" variant={ButtonVariant.SECONDARY} />
          <Button label="Save connection" type="submit" />
        </Flex>
      </Flex>
    </form>
  );
}
```

- **Labels.** Every control has a visible label (`label` on the input or the wrapping `Field`, never both). A placeholder is an example, never the label. Labels are nouns in sentence case: "Connection name", not "Enter connection name".
- **Descriptions** say what the value is for or its format, in one line. No buttons in descriptions; a link at the end of the label row goes in `Field actions`.
- **Required and optional.** Mark whichever is rarer: `isRequired` when most fields are optional, `isOptional` when most are required. Never rely on the asterisk alone in a group: say it in the `Fieldset` description too.
- **Validation lives in the app.** No DLS control validates; compute the message and pass `error`.
- **Timing.** Validate on submit (and on blur for a field the user has left with a clearly wrong format). After a field shows an error, re-validate it on every change so the error clears as soon as it is fixed. Don't show errors on fields the user has not touched.
- **Error messages** say what to do: "Enter a connection name.", "Use 100 characters or fewer.", "Port must be between 1 and 65535." Not "Invalid input", not "Error: required".
- **Group** related controls with `Fieldset`; a form with sections is a column of `Fieldset`s with `gap={24}`.
- **Actions** sit at the end of the form, end-aligned: Cancel (`SECONDARY`) then the submit (`PRIMARY`, `type="submit"`, a verb plus object: "Save connection"). `isLoading` on the submit while it saves.
- **Settings that apply immediately** use `SwitchInput` and save on change (with `isLoading` while in flight); forms that save on submit use `CheckboxInput`. Don't mix both models in one form.
- **Ghost fields** (`isGhost`) are for toolbars, table cells and inline editing, where a frame at rest would be noise. Forms use framed fields.
- **Horizontal fields** (`Field orientation={HORIZONTAL}`) for settings pages with short controls; vertical (default) everywhere else.

## Tables and data views

- **Pick the table:** `InfiniteTable` for read-only data in memory, `VirtualizedInfiniteTable` for very long server-paged lists, `InfiniteSpreadsheet` for editing in place. Key–value data of one record is a `DescriptionList`.
- **Bound the parent** (`Box height`, a grid track or a flex column with `minHeight={0}`): the table scrolls inside it.
- **The last row closes itself.** Its bottom hairline is drawn while the rows end above the table's bottom edge (a full-height table with a few rows) and dropped when the last row lands on that edge, where the container's frame draws the line. There is no prop for it; don't draw your own line under a table.
- **Always pass `getRowId`** when rows can be sorted, inserted or removed; selection and the active row follow ids.
- **Row header:** mark the column that names the row `isRowHeader`; it labels the checkbox and row actions for screen readers.
- **Numbers** are right-aligned (`align: "right"`) and tabular; ids, hashes and timestamps are mono.
- **Row actions** go in `rowActions` (a "…" menu). A row click (`onRowClick`) opens the record; set `activeRowId` while its panel is open. Don't put buttons in cells for actions the menu can hold.
- **Selection** (`isSelectable`) when there are bulk actions; show the bulk actions in the table toolbar while something is selected ("3 selected · Delete").
- **Toolbar** above the table: a `SearchInput` (`onSearch`), filter `Dropdown`s or filter `Chip`s, then the primary action at the end, all `SMALL`.
- **Loading:** `isLoading` shows skeleton rows (first load) or a loading row (more). Don't put a `Spinner` over a table.
- **Empty:** `emptyState` with an `EmptyState` that says what would be here and offers the action that adds it. A filtered empty result says so and offers to clear the filters (`role="status"`).
- **Error:** `error` shows an error state (empty body) or an error row (after the last row); loading more pauses until it clears. Pair it with a retry.
- **Pagination vs infinite:** paginate (`Pagination`) when users need to reference a position ("page 3"), jump, or the total is meaningful; load more on scroll (`onEndReached`) for feeds and logs. Don't paginate a list that fits on one page. Reset to page 1 when filters change.
- **Columns:** let users resize and arrange columns (`canResizeColumns`, `canCustomizeColumns`) on data-heavy tables, and persist `columnLayout`.

## Overlays

- **Choose by job:** a hint is a `Tooltip`; interactive anchored content is a `Popover`; a panel with Apply / Reset is a `Dropdown`; commands are a `Menu`; a blocking task is a `Modal`; a yes/no before an action is a `ConfirmDialog`; detail beside a list is a `Drawer`; app-wide commands are the `CommandPalette`.
- **Focus.** Overlays move focus in, trap it (Modal, Drawer, CommandPalette), and restore it to the trigger on close. Give the first field `autoFocus` in a form dialog.
- **Escape** closes the topmost overlay only. Don't intercept Escape in content inside an overlay unless it consumes it (an open select).
- **Stacking.** The overlay stack orders layers: dropdowns < drawers < modals < toasts < tooltips. A Menu inside a Modal opens above it. Avoid a Modal on a Modal; replace the content or use a Drawer.
- **Keep them mounted.** Drive Modal, Drawer and ConfirmDialog with `isOpen` instead of rendering them conditionally, so they animate out and restore focus.
- **Dialog structure:** a `header` that names the task ("Rename pipeline"), the body, a `footer` with the least destructive action first and the primary last.
- **Drawer actions split by job.** Actions on the thing a drawer shows (open in full page, copy link, re-run) go in its title row (`actions`): one or two icon-only buttons, then a ⋯ `Menu` for the rest, with Delete last and red. The `footer` only finishes the drawer's task (Cancel, then Save); a read-only detail drawer usually has none. Never put ⋯ in the footer.
- **Overlay titles: header, subheader, icon.** A Modal or Drawer title is `header`, with `subheader` for the second line (the connector, the run's pipeline) and `icon` for a leading glyph; don't build them into a custom `header` node. A node `header` renders in a first line one control tall, where the close button and `actions` center; anything taller pushes below that line.
- **Tooltips** hold short hints, never essential information or controls. Every icon-only button gets a tooltip with its accessible name.

## Destructive actions

- Use `ButtonVariant.ERROR` only for actions that delete or cannot be undone, and `MenuItemVariant.ERROR` for those rows in a menu. Nothing else is red.
- Confirm with `ConfirmDialog isDestructive`: the header asks the question with the object's name ("Delete orders_db?"), the description says what happens and what does not, the button names the action ("Delete source", never "OK" or "Yes").
- For high-impact deletions (a workspace, a production source), add `confirmValue` (type the name to confirm).
- Prefer undo over confirmation when the action is reversible: do it, then show a `Toast` with an "Undo" action.
- Let `onConfirm` return the promise: the dialog shows a spinner, stays open on failure, and the app sets `error`.

## Feedback

| Situation | Use |
|---|---|
| The user did something and it worked (Saved, Copied, Invited) | `Toast` (`SUCCESS` or `PRIMARY`), or nothing if the UI already shows the result |
| The user did something reversible (Archived, Removed) | `Toast` with an "Undo" `action` |
| Something the user did failed | Inline `error` on the field, or an `Alert` in the form or dialog; a `Toast` (`ERROR`) only when there is no place on screen |
| A lasting condition on the page (a source is failing, a plan limit is near) | `Alert` in the page or card, or `isBanner` across the page |
| The live state of one item in a list | `Beacon label` |
| A status value users filter or scan (Failed, Paused) | `Chip` with a status `variant` |

- **Toasts** are transient, one per event, one short line, never the only record of an outcome. Use `toast.promise` for async work.
- **Alerts** stay until the condition is resolved; `ERROR` alerts give a way out (a retry or a link). The way out is one `PRIMARY` `SMALL` button, any other action `TERTIARY`. Don't stack several alerts about one thing.
- **Statuses** use the status variants only: success, warning, error. Neutral news is `PRIMARY`.
- **Loading:** a `Skeleton` where the content's shape is known (the default for pages, cards and tables); a `Spinner` where it is not, or in a small space; a `ProgressBar` / `ProgressCircle` when the total is known; `isLoading` on the button that started it. Set `aria-busy` on the region. A whole region waiting on its first load takes `PendingLayout`.
- Don't block the whole page for a partial load; load regions independently.

## Empty states

- Say what would be here and give the action that puts it there: "No sources yet" + "Connect a database or an app to start syncing data." + `Connect source`.
- Size by placement: `SMALL` in a cell, chart or menu; `MEDIUM` in a card; `LARGE` for a whole page.
- One `PRIMARY` and at most one `SECONDARY` action (or a `Link` to docs).
- A search or filter with no results: "No results for “orders”", a description that suggests changing the search, and a "Clear filters" action; `role="status"`.
- A whole region (a page body, a panel) that is empty, failed or loading takes `EmptyLayout`, `ErrorLayout` or `PendingLayout`, which fill it and centre the state.
- A region that failed to load is an `ErrorLayout` with a retry, not an empty state. A problem with content that stays on screen is an `Alert`.
- Charts and tables have `placeholder` / `emptyState`; use them instead of overlaying your own.

## Charts

- **Pick by question:** trend over time → `LineChart`; volume or parts of a total over time → `AreaChart`; compare categories → `BarChart`; parts of one whole (two to six) → `PieChart`; two categorical dimensions → `Heatmap`; a trend beside a number → `Sparkline`; a KPI → `BigNumber`, or a `BigNumberGroup` for a lead KPI and its breakdown.
- **Labels.** Give every chart an `ariaLabel` that states what it shows and the trend ("Signups, last 30 days, rising"). Name series in `series.label`; title axes with `axisLabels` when the unit is not obvious; format values with `valueFormatter` (`formatNumber`, `formatPercent`, `formatBytes`).
- **Small sizes.** Charts thin their ticks and labels to fit; at card size, hide the legend (`hasLegend={false}`) when the card header names the single series. Below about 120 × 80px, use a `Sparkline`.
- **Color.** Let series take the automatic palette order. Use a status slot (`ChartPalette.ERROR`) only when the series *is* a status (failures). Use one hue in a `Heatmap` whose meaning fits the value.
- **Never color-only meaning.** Every series has a legend entry and a tooltip label; a status shown by color also shows its word.
- **Linked charts** that share a category axis go in a `ChartGroupProvider`.
- **Filtering.** When clicking a chart should narrow it (or a table beside it), set `isFilterable` rather than wiring `onSelect` to your own `selection`; control it with `selection` + `onSelectionChange` only when something else on the page reads the filter.
- Size charts with their parent (a `Box`, a `Grid` track); don't pass pixel sizes.

## Color

- **Monochrome by default.** Surfaces are neutral steps; text is `primary`, `secondary`, `tertiary`; the primary action, selection, focus and links are neutral. There is no brand color to add.
- **Status color for status only:** success, warning, error, through component `variant`s.
- **Categorical color for categories only:** a team, a source type, a chart series, through the `color` prop. One category keeps its color everywhere.
- Never use color to decorate (a hue per chip, a colored card) and never as the only signal.
- Don't write hex. `hex` props are for colors that are data (a user-picked label color).

## Typography hierarchy

| Role | Text |
|---|---|
| Page title | `HEADING_MD`, `MEDIUM`, `as="h1"` |
| Section title, card title | `HEADING_SM` or `BODY_LG`, `MEDIUM`, `as="h2"` / `h3` (Widget renders its own) |
| Body, values, labels | `BODY_MD` (default) |
| Supporting copy, descriptions | `BODY_MD` or `BODY_SM`, `SECONDARY` |
| Metadata, timestamps, hints | `BODY_SM` or `CAPTION`, `TERTIARY` |
| Small section labels | `CAPTION`, `TERTIARY`, `transform={TextTransform.UPPERCASE}` |
| Ids, hashes, code, numbers in tables | `family={FontFamily.MONO}` |
| Long-form reading | `isProse` (or `MarkdownText`) |
| Editorial, marketing-like copy | `family={FontFamily.SERIF}`, `BODY_LG` and up |

- Two weights only: regular for everything, medium for titles and emphasis (`Bold`).
- Hierarchy comes from size, weight and the text role, not from color or extra weights.
- Truncate single-line UI text with `lineClamp={1}` and `shouldTooltipOnOverflow`.
- Never style raw `<p>` / `<span>` / `<h1>`: use `Text` and `Span` (the Biome preset warns).

## Motion

- Motion is functional: it shows where something came from or that something changed. 75ms for color and border, 150ms for enter, exit and transforms.
- Overlays, toasts and disclosures animate themselves. Overlay panels leave content first: the content fades in place over 75ms, then the empty panel scales, slides or collapses over the next 75ms, so text is never scaled, smeared or squeezed on the way out. For your own content use `Fade` (appear in place), `ScaleFade` (floating surfaces), `Rotate` (carets), `Flasher` (a value updating).
- No decorative animation, no bounces, no motion longer than 150ms in product UI (brand animations are the exception, in hero and splash moments).
- Everything stops under reduced motion (`prefers-reduced-motion` or `GalaxyProvider reducedMotion`); don't add motion that ignores it.

## Keyboard and accessibility

- Every action is reachable by keyboard; every icon-only control has an `ariaLabel` (the types require it) and a tooltip.
- Use the component that has the right pattern instead of building one: `Tabs` (arrows), `Menu` (arrows, typeahead), `Tree`, `ToggleInput`, `RadioGroup`, `SelectInput`.
- Shortcuts: show them with `hotKeys` (Button, Tooltip, MenuItem, CommandPalette items) and bind them with `shouldBindHotKey` or `useHotkey`. Use `mod` for ⌘ / Ctrl. Single-key shortcuts must not fire in text fields (`shouldIgnoreInputs`).
- Context menus and hover popovers are shortcuts, never the only way to an action.

## Writing

- **Sentence case** everywhere: titles, buttons, labels, menu items ("Run sync", not "Run Sync").
- **Buttons are verbs**, plus the object when it is not obvious: "Save connection", "Delete source", "Invite members". Not "OK", "Submit", "Yes".
- **No "please", no exclamation marks**, no "successfully" ("Source saved", not "Source was saved successfully!").
- **Be specific:** name the object ("Delete orders_db?") and the consequence ("Its 12 tables stop syncing.").
- **Errors** say what happened and what to do, without blame: "Could not connect to db.internal. Check the host and port." Not "Error 500", not "You entered an invalid host".
- **Empty states** say what would be here and how to add it.
- **Numbers and dates** through `utils/format` (`formatNumber`, `formatRelativeTime`) so they read the same everywhere; ellipsis `…` for "in progress" labels ("Saving…").
- Use the product's nouns consistently (source, pipeline, run, workspace).

## Dark and light parity

- Build and review every screen in both themes (Storybook's Theme toolbar; `ThemeSwitcher` in the app). Both themes define every token, so a screen that uses tokens works in both.
- Never pick a color for one theme (a hex, a `PALETTE` step in UI). A surface that must contrast with the page is an inverse scope (a scoped `GalaxyProvider`), not hand-picked dark colors.
- Images and illustrations need a variant per theme or a neutral treatment; brand marks follow the theme on their own.
- Check states in both: hover, selected, focus, disabled, error.

## Do and don't

| Do | Don't |
|---|---|
| One `PRIMARY` button per view | Several `PRIMARY` buttons competing |
| Cards as `Widget` | A bordered, rounded `Box` with a heading |
| `gap` and `padding` from the scale | Margins, or `gap={10}` |
| `Text` / `Span` for all copy | Raw `<p>`, `<span>`, inline font styles |
| Status `variant` for status, `color` for category | A hue to make something stand out |
| `ConfirmDialog` with a named action for deletes | "Are you sure?" with OK / Cancel |
| `Toast` for a transient result, `Alert` for a lasting condition | A toast for an error the user must fix |
| `Skeleton` in the shape of the content | A full-page spinner |
| `SMALL` controls in toolbars and the `Topbar` | Mixed rungs in one row |
| `getRowId` on every table | Selection keyed by index |
| Visible labels on every field | Placeholder as label |
| Both themes checked | A screen designed in one theme |
