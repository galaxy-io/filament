<!-- Generated from docs/components.md by `pnpm skill:sync`. Edit that file, not this one. -->

# Components

Every module `@galaxy-io/dls` exports, grouped by job. For each: what it is for, when to use it and when to reach for a sibling, the props that matter, and a minimal example. Props, defaults and every variant are in Storybook (each prop's JSDoc is its documentation).

- Import each module from its subpath; the default export is the component, named exports are its enums and types.
- Shared conventions (sizes, variants, `color`, booleans, state triples, slots) are in [AGENTS.md](https://github.com/galaxy-io/dls/blob/main/AGENTS.md#component-conventions); tokens in [theming.md](./tokens.md); how to combine components in [patterns.md](./patterns.md).
- Every example compiles against the current package (`pnpm skill:check`). Icons come from `@phosphor-icons/react`.

**Contents:** [Layout](#layout) · [Typography](#typography) · [Actions](#actions) · [Forms](#forms) · [Data display](#data-display) · [Feedback](#feedback) · [Overlays](#overlays) · [Navigation](#navigation) · [Tables](#tables) · [Charts](#charts) · [Editor](#editor) · [Brand](#brand) · [Transforms](#transforms) · [Theme and setup](#theme-and-setup) · [Hooks and utils](#hooks-and-utils) · [Accessibility primitives](#accessibility-primitives) · [Low-level modules](#low-level-modules)

## Layout

| You need | Use | Not |
|---|---|---|
| A row or column of things with a gap | `Flex` | `Box` with CSS, margins |
| Two-dimensional tracks (a form grid, a dashboard) | `Grid` | nested `Flex` rows of fixed widths |
| Padding, a surface step, a size, scrolling or position around content | `Box` | `Widget` (no header, no card) |
| A titled or bordered unit of content: a card | `Widget` | a bordered, rounded `Box` with a heading |
| Several collapsible cards where opening one closes the others | `WidgetGroup` | a `WidgetGroup` around unrelated cards (use `Flex gap={8}`) |
| One flex child with its own grow / shrink / basis | `FlexItem` | `FlexItem` when `Flex` props on the parent are enough |
| A hairline between regions | `Divider` | a bordered `Box` |
| A scrolling region with quiet scrollbars | `ScrollArea` | `overflow: auto` with native scrollbars |
| User-resizable split panes | `ResizablePanels` | hand-wired `ResizeHandle`s |

### Box

`@galaxy-io/dls/layout/Box` · The generic block container: `variant` (surface step; unset is no fill), `hasBorder`, `radius`, `padding`, sizing (`width`, `height`, `min*`, `max*`, `fillWidth`), `overflow`, `position` + `inset`, `as`.

- Use for page and panel padding, a toolbar surface, sizing a chart or table parent, positioning.
- Don't use for cards (use `Widget`) or clickable regions (Box takes no event handlers on purpose).

```tsx
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Text from "@galaxy-io/dls/text/Text";

<Box variant={BoxVariant.SECONDARY} padding={[8, 16]} fillWidth>
  <Text>Filters apply to every chart on this page.</Text>
</Box>;
```

### Flex

`@galaxy-io/dls/layout/Flex` · Flexbox: `direction`, `alignItems`, `justifyContent`, `wrap` (enums from the module), `gap` (one value or `[row, column]`), `padding`, sizing, `grow` / `shrink` / `basis` when it is itself a flex child, `overflow`, `as`.

- The default layout primitive. There is no `Stack`: a column is `direction={FlexDirection.COLUMN}`.
- Pass `minWidth={0}` on a child column so nested text can truncate.

```tsx
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

<Flex direction={FlexDirection.COLUMN} gap={12} padding={24}>
  <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
    <Text as="h1">Sources</Text>
    <Button label="New source" />
  </Flex>
  <Text variant={TextVariant.SECONDARY}>Connect a database or an app.</Text>
  <Button label="Cancel" variant={ButtonVariant.SECONDARY} />
</Flex>;
```

### FlexItem

`@galaxy-io/dls/layout/FlexItem` · One child of a `Flex` with its own `grow`, `shrink`, `basis` and sizing. A plain block: nest a `Flex` inside to arrange its children.

```tsx
import Flex from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text from "@galaxy-io/dls/text/Text";

<Flex gap={16}>
  <FlexItem basis={240} shrink={0}>
    <Text>Fixed column</Text>
  </FlexItem>
  <FlexItem grow={1} minWidth={0}>
    <Text lineClamp={1}>Fluid column that truncates</Text>
  </FlexItem>
</Flex>;
```

### Grid

`@galaxy-io/dls/layout/Grid` · CSS grid: `columns` / `rows` (a number of equal tracks or a template string), `areas`, `gap`, four alignment enums, `padding`, sizing.

```tsx
import Grid from "@galaxy-io/dls/layout/Grid";
import StatChart from "@galaxy-io/dls/charts/StatChart";

<Grid columns={3} gap={12}>
  <StatChart label="Rows synced" value="48,210" hasBorder />
  <StatChart label="Sources" value={12} hasBorder />
  <StatChart label="Failed runs" value={0} hasBorder />
</Grid>;
```

### Divider

`@galaxy-io/dls/layout/Divider` · The hairline rule: `orientation` (`Orientation` from `theme/enums`), an optional centered `label` on a horizontal rule. Adds no space; the parent's `gap` spaces it. A vertical rule needs a flex or grid parent.

```tsx
import Divider from "@galaxy-io/dls/layout/Divider";

<Divider label="or" />;
```

### Widget

`@galaxy-io/dls/widget/Widget` · The card: a hairline-bordered surface with an optional header row (`header`, `subheader`, leading `icon`, `actions`), a body and a `footer`, separated by hairlines. `variant` (surface set, default `PRIMARY`), `size` (`SMALL MEDIUM LARGE` density), `gap` (space between body children, default the size's 8 / 12 / 16; `0` stacks flush sections edge to edge), `isFlush` (body runs to the edges: tables, charts, lists), `isLoading`, `isSelected`. Modes: `isCollapsible` (the header is a disclosure button; `isOpen` / `defaultIsOpen` / `onOpenChange`, `hasCaret`) or `isInteractive` + `onClick` (the whole card is one button).

- Any titled or bordered unit of content is a Widget, even one without a header.
- Header `actions` are `ButtonSize.SMALL` buttons, a count `Chip` or a `Menu` trigger. No buttons in the body of an interactive card.
- No hue cards: put an `Alert` or a labeled `Beacon` inside a neutral Widget.

```tsx
import { DatabaseIcon } from "@phosphor-icons/react";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

<Widget
  header="Postgres"
  subheader="orders_db · synced 2 min ago"
  icon={DatabaseIcon}
  actions={<Button label="Sync" size={ButtonSize.SMALL} variant={ButtonVariant.SECONDARY} />}
>
  <Text variant={TextVariant.SECONDARY}>48,210 rows in 12 tables.</Text>
</Widget>;
```

### WidgetGroup

`@galaxy-io/dls/widget/WidgetGroup` · Coordinates collapsible `Widget`s (each with an `id`): one open at a time unless `canOpenMultiple`, arrow keys between headers. Open ids are `value` / `defaultValue` / `onChange`. Replaces an accordion.

```tsx
import Text from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";
import WidgetGroup from "@galaxy-io/dls/widget/WidgetGroup";

<WidgetGroup defaultValue={["general"]}>
  <Widget id="general" header="General" isCollapsible>
    <Text>Name, owner and schedule.</Text>
  </Widget>
  <Widget id="advanced" header="Advanced" isCollapsible>
    <Text>Retries and timeouts.</Text>
  </Widget>
</WidgetGroup>;
```

### ScrollArea

`@galaxy-io/dls/layout/ScrollArea` · A scrolling region with thin overlay scrollbars (the reset hides native ones). Fills the height its parent gives it; `canScrollHorizontally`, `hasFadeEdges`; `ariaLabel` makes it a named region.

- Bound the parent; put padding on a `Box` inside, not on the ScrollArea; don't nest two on the same axis.

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text from "@galaxy-io/dls/text/Text";

<Box height={240}>
  <ScrollArea hasFadeEdges ariaLabel="Run log">
    <Box padding={12}>
      <Text>Long content…</Text>
    </Box>
  </ScrollArea>
</Box>;
```

### ResizablePanels

`@galaxy-io/dls/layout/ResizablePanels` · Lays out `ResizablePanel` children along `orientation` with a `ResizeHandle` between each pair. Exactly one panel is fluid (no `value` / `defaultValue`); every sized panel has an `ariaLabel`, `min` and `max`. `storageKey` persists sizes.

- Use for an editor beside a preview, a list beside its detail that the user resizes.
- Don't use for a panel that opens on demand (that is a `Drawer isModal={false}`).

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import ResizablePanels, { ResizablePanel } from "@galaxy-io/dls/layout/ResizablePanels";
import Text from "@galaxy-io/dls/text/Text";

<Box height={400}>
  <ResizablePanels storageKey="pipeline-editor">
    <ResizablePanel id="files" defaultValue={240} min={160} max={400} ariaLabel="Files">
      <Text>Files</Text>
    </ResizablePanel>
    <ResizablePanel id="editor">
      <Text>Editor</Text>
    </ResizablePanel>
  </ResizablePanels>
</Box>;
```

### ResizeHandle

`@galaxy-io/dls/layout/ResizeHandle` · The grab bar between two panes when you manage sizes yourself: `value` (the pane's size in px) + `onChange`, `min`, `max`, `orientation`, `isReversed`, required `ariaLabel`. Drag or arrow keys. Prefer `ResizablePanels`.

```tsx
import { useState } from "react";
import ResizeHandle from "@galaxy-io/dls/layout/ResizeHandle";
import { Orientation } from "@galaxy-io/dls/theme/enums";

export function SidebarHandle() {
  const [width, setWidth] = useState(280);
  return (
    <ResizeHandle
      orientation={Orientation.VERTICAL}
      value={width}
      onChange={setWidth}
      min={200}
      max={480}
      ariaLabel="Resize sidebar"
      aria-controls="sidebar"
    />
  );
}
```

### Portal

`@galaxy-io/dls/layout/Portal` · Renders children into the provider's `portalContainer` (else `document.body`) while they stay in place in the React tree. Overlays already use it; reach for it only for your own floating layer. `container` overrides the node; `isDisabled` renders inline.

```tsx
import Portal from "@galaxy-io/dls/layout/Portal";
import Text from "@galaxy-io/dls/text/Text";

<Portal>
  <Text>Rendered at the end of the body.</Text>
</Portal>;
```

### GridBackground

`@galaxy-io/dls/backgrounds/GridBackground` · A quiet dot, cross or cell grid (`pattern`, `GridBackgroundPattern`) behind centered children, at a pitch of 8 / 12 / 24 / 32 / 48px (`size`, `GridBackgroundSize`). Decoration only: empty canvases, hero areas. The parent `Box` gives the surface and clipping. To tone the marks, set `GRID_BACKGROUND_COLOR_VAR` (a `t.color.*` token) and `GRID_BACKGROUND_OPACITY_VAR` (0 to 1) on `style`.

```tsx
import GridBackground, { GridBackgroundPattern } from "@galaxy-io/dls/backgrounds/GridBackground";
import Box from "@galaxy-io/dls/layout/Box";
import Text from "@galaxy-io/dls/text/Text";

<Box height={200} hasBorder overflow="hidden">
  <GridBackground pattern={GridBackgroundPattern.CROSS}>
    <Text>Drop a node here</Text>
  </GridBackground>
</Box>;
```

```tsx
import GridBackground, {
  GRID_BACKGROUND_COLOR_VAR,
  GRID_BACKGROUND_OPACITY_VAR,
  GridBackgroundSize,
} from "@galaxy-io/dls/backgrounds/GridBackground";
import { t } from "@galaxy-io/dls/theme/tokens/t";

<GridBackground
  size={GridBackgroundSize.X_SMALL}
  style={{
    [GRID_BACKGROUND_COLOR_VAR]: t.color.text.tertiary,
    [GRID_BACKGROUND_OPACITY_VAR]: 0.4,
  }}
/>;
```

## Typography

| You need | Use |
|---|---|
| Any block of text (a paragraph, a heading, a label) | `Text` (`as="h1"` … for semantics) |
| Formatting part of a `Text` (color, weight, mono, italic) | `Span` |
| Medium-weight emphasis inside a `Text` | `Bold` (a `Span` preset) |
| An identifier or literal inside a sentence | `Code` (a `Span isCode` preset) |
| A part of chrome text the user may select and copy | `Selectable` (a `Span isSelectable` preset) |
| Read-only multi-line code | `CodeBlock` |
| Editable code | `CodeEditor` |
| Markdown content from data | `MarkdownText` |
| A keyboard shortcut | `Kbd` |
| Click-to-rename text | `EditableText` |
| A link | `Link` |
| A short bulleted list | `BulletedList` |

**Text vs Span.** `Text` owns the metrics (size, line height, family) and renders a block (`<p>` by default). `Span` lives inside a `Text`, inherits its size, family and color, and changes only what you pass. Never nest a `Text` in a `Text` to change a word's color.

### Text

`@galaxy-io/dls/text/Text` · The text primitive. `variant` (`TextVariant` color role, default `PRIMARY`), `size` (`TextSize` step, default `BODY_MD`), `weight` (`REGULAR` / `MEDIUM`), `family` (`FontFamily`), `as` (the element; heading semantics never change the style), `lineClamp` + `shouldTooltipOnOverflow`, `isProse` (1.5 line height, selectable), `isTabular`, `transform` (`UPPERCASE` for small labels), `align`.

```tsx
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

<>
  <Text as="h2" size={TextSize.HEADING_SM} weight={TextWeight.MEDIUM}>
    Schedule
  </Text>
  <Text variant={TextVariant.SECONDARY}>Runs every day at 02:00 UTC.</Text>
  <Text family={FontFamily.MONO} lineClamp={1} shouldTooltipOnOverflow>
    postgres://orders.internal:5432/orders_db
  </Text>
</>;
```

### Span, Bold, Code, Selectable

`@galaxy-io/dls/text/Span` · Inline formatting inside a `Text`: `variant` (default `INHERIT`), `weight`, `family`, `isItalic`, `isUnderlined`, `isStrikethrough`, `isSelectable`, `isCode`. Presets with the same props: `@galaxy-io/dls/text/Bold` (medium weight), `@galaxy-io/dls/text/Code` (inline code chip), `@galaxy-io/dls/text/Selectable` (selectable text).

```tsx
import Bold from "@galaxy-io/dls/text/Bold";
import Code from "@galaxy-io/dls/text/Code";
import Selectable from "@galaxy-io/dls/text/Selectable";
import Span, { SpanVariant } from "@galaxy-io/dls/text/Span";
import Text from "@galaxy-io/dls/text/Text";

<Text>
  <Bold>orders_db</Bold> failed at <Code>COPY orders</Code>:{" "}
  <Span variant={SpanVariant.ERROR}>permission denied</Span>. Contact{" "}
  <Selectable>data@acme.com</Selectable>.
</Text>;
```

### CodeBlock

`@galaxy-io/dls/text/CodeBlock` · Read-only multi-line code: `content`, `language` (`CodeBlockLanguage`), `label` (a file name), `canCopy`, `hasLineNumbers`, `isDiff`. Cap the height with a parent `Box maxHeight`.

```tsx
import CodeBlock, { CodeBlockLanguage } from "@galaxy-io/dls/text/CodeBlock";

<CodeBlock
  content={"select id, email\nfrom users\nwhere created_at > now() - interval '7 days';"}
  language={CodeBlockLanguage.SQL}
  label="recent_users.sql"
  canCopy
/>;
```

### MarkdownText

`@galaxy-io/dls/text/MarkdownText` · GitHub-flavored Markdown as DLS typography (prose `Text`, headings, `Code`, `Link`, `CodeBlock` fences, hairline tables). `family={FontFamily.SERIF}` for editorial prose. Raw HTML is dropped unless `shouldRenderHtml` (trusted content only).

```tsx
import MarkdownText from "@galaxy-io/dls/text/MarkdownText";

<MarkdownText content={"## Release notes\n\n- Faster syncs\n- `orders` is now incremental"} />;
```

### Kbd

`@galaxy-io/dls/text/Kbd` · Key caps for a shortcut. `hotKeys` in `useHotkey` grammar (`["mod", "k"]` shows ⌘ K on Mac, Ctrl K elsewhere). `size` matches the rung or text step beside it; `isPlain` for caps on a solid host. Binds nothing; Button, Tooltip, Menu items and SearchInput take `hotKeys` directly.

```tsx
import Kbd from "@galaxy-io/dls/text/Kbd";

<Kbd hotKeys={["mod", "shift", "p"]} />;
```

### EditableText

`@galaxy-io/dls/text/EditableText` · Click-to-rename: reads as `Text`, becomes a frameless field in the same type on click or Enter. Enter or blur commits (`onChange` only when changed), Escape cancels. `ariaLabel` is required ("Pipeline name").

```tsx
import EditableText from "@galaxy-io/dls/text/EditableText";
import { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";

<EditableText
  defaultValue="Nightly orders sync"
  ariaLabel="Pipeline name"
  size={TextSize.HEADING_SM}
  weight={TextWeight.MEDIUM}
  onChange={(name) => console.log(name)}
/>;
```

### Ellipsis

`@galaxy-io/dls/text/Ellipsis` · Three dots pulsing after a label ("Generating…"). Not truncation (that is `Text lineClamp`). Static under reduced motion.

```tsx
import Ellipsis from "@galaxy-io/dls/text/Ellipsis";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

<Text variant={TextVariant.SECONDARY}>
  Generating summary
  <Ellipsis />
</Text>;
```

### RequiredMarker

`@galaxy-io/dls/text/RequiredMarker` · The error-colored asterisk beside a required label. `Field` and the inputs render it from `isRequired`; use it directly only in a custom label.

```tsx
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text from "@galaxy-io/dls/text/Text";

<Text as="label">
  Workspace name <RequiredMarker />
</Text>;
```

### Link

`@galaxy-io/dls/links/Link` · Inline navigation: `href`, `as` (your router link), `isExternal` (new tab plus arrow), `variant` (`NEUTRAL` / `INHERIT`), `underline` (`ALWAYS` / `HOVER` / `NONE`), `isActive`, `size` for a standalone link. Without `href` it renders a non-interactive span for use inside a clickable parent.

- A Link goes somewhere. An action is a `Button` (`ButtonVariant.TERTIARY` for a quiet one).

```tsx
import Link from "@galaxy-io/dls/links/Link";
import Text from "@galaxy-io/dls/text/Text";

<Text>
  Read the <Link href="https://docs.getgalaxy.io/sync" isExternal>sync guide</Link> first.
</Text>;
```

### BulletedList

`@galaxy-io/dls/lists/BulletedList` · A short unordered list: `items` (strings or inline content, may end in a nested list), `size` (`SMALL MEDIUM LARGE`).

```tsx
import BulletedList from "@galaxy-io/dls/lists/BulletedList";

<BulletedList items={["Create a read-only user", "Allow our IP range", "Paste the connection string"]} />;
```

### Icon

`@galaxy-io/dls/icons/Icon` · The one way to draw a Phosphor icon: `component` (the icon component), `size` (px; parents pick it from their rung), `variant` (`IconVariant`, the text roles), `weight` (`IconWeight`), `ariaLabel` (decorative without it), `tooltip`, `isSpinning`. Components with an `icon` prop render through it; never render `<PlusIcon />` directly.

```tsx
import { WarningIcon } from "@phosphor-icons/react";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

<Icon component={WarningIcon} size={14} variant={IconVariant.WARNING} ariaLabel="Degraded" />;
```

## Actions

| You need | Use | Not |
|---|---|---|
| A command | `Button` | `Link` (a link goes somewhere), a clickable `Chip` |
| The main action of a view | `Button` `PRIMARY` (one per view) | several `PRIMARY` buttons |
| A main action with secondary options | `Button dropdown` (split button) | an attached `ButtonGroup` |
| A row of related buttons | `ButtonGroup` | `Flex gap={8}` when the buttons are not one group |
| Copy a value | `CopyButton` (or `CopyInput` to show the value too) | `useClipboard` + `Button` by hand |
| Pick one of a few options | `ToggleInput` | attached buttons with `isActive` |

**Button modes and sizes**

| `ButtonVariant` | Look | Use |
|---|---|---|
| `PRIMARY` (default) | Monochrome solid (`solid.primary`) | The one main action of a view: Save, Create, Run. |
| `SECONDARY` | Bordered | Every other action next to it: Cancel, Export, Edit. |
| `TERTIARY` | Transparent until hovered | Low-emphasis and icon-only actions: toolbars, rows, in-field buttons. |
| `BASE` | Muted fill (`solid.neutral`), no border | A quiet filled action on `base` / `primary` surfaces. Not on a `background.secondary` surface (it disappears). |
| `ERROR` | Red solid (`solid.error`) | Destructive actions only: Delete, Revoke. Usually inside a `ConfirmDialog`. |

| `ButtonSize` | Height | Where |
|---|---|---|
| `X_SMALL` | 20px | Inside dense rows, chips, table cells. |
| `SMALL` | 24px | Toolbars, `Topbar`, `Widget` headers, inline actions, `Alert` actions. |
| `MEDIUM` (default) | 32px | Forms, dialogs, page actions. |
| `LARGE` | 36px | Prominent standalone actions, onboarding. |
| `X_LARGE` | 40px | Hero and empty-state calls to action on marketing-like pages. |

### Button

`@galaxy-io/dls/buttons/Button` · The action control. `label`, `icon` (+ `isIconTrailing`), `variant`, `size`, `onClick` or `type` (form) or `href` (+ `as`, `isExternal`), `isLoading` (spinner, keeps the width, blocks activation), `isDisabled`, `isActive` (toggle look, `aria-pressed`), `isRound`, `fillWidth`, `leading` / `trailing` (non-interactive), `hotKeys` (+ `shouldBindHotKey`), `tooltip`, `dropdown` (split button).

- Icon-only buttons need `ariaLabel` (a type error otherwise); add a `tooltip` with the same words.
- `isLoading` while the action runs; don't disable and swap the label.

```tsx
import { ArrowsClockwiseIcon, TrashIcon } from "@phosphor-icons/react";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex from "@galaxy-io/dls/layout/Flex";

<Flex gap={8}>
  <Button label="Run sync" icon={ArrowsClockwiseIcon} isLoading={false} onClick={() => {}} />
  <Button label="Edit" variant={ButtonVariant.SECONDARY} />
  <Button
    icon={TrashIcon}
    ariaLabel="Delete source"
    tooltip="Delete source"
    variant={ButtonVariant.TERTIARY}
    size={ButtonSize.SMALL}
  />
</Flex>;
```

### ButtonGroup

`@galaxy-io/dls/buttons/ButtonGroup` · Related buttons as one group (`role="group"`, `ariaLabel`). `isAttached` joins `SECONDARY` buttons of one size into a segmented row; `orientation`; `fillWidth`. Passes nothing to its children.

- Don't attach `PRIMARY` or `ERROR`; don't mix sizes; don't use attached buttons to pick one option (that is `ToggleInput`).

```tsx
import { TextBIcon, TextItalicIcon } from "@phosphor-icons/react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import ButtonGroup from "@galaxy-io/dls/buttons/ButtonGroup";

<ButtonGroup isAttached ariaLabel="Text formatting">
  <Button icon={TextBIcon} ariaLabel="Bold" variant={ButtonVariant.SECONDARY} />
  <Button icon={TextItalicIcon} ariaLabel="Italic" variant={ButtonVariant.SECONDARY} />
</ButtonGroup>;
```

### CopyButton

`@galaxy-io/dls/buttons/CopyButton` · Copies `value` and confirms (check icon, "Copied" tooltip, live announcement). Button's `variant` (no `ERROR`, default `TERTIARY`) and `size`. Name what is copied: `label` or `ariaLabel` ("Copy API key").

```tsx
import CopyButton from "@galaxy-io/dls/buttons/CopyButton";

<CopyButton value="sk_live_51H…" ariaLabel="Copy API key" />;
```

## Forms

| The answer is | Use | Not |
|---|---|---|
| Free text, one line | `TextInput` | `TextAreaInput` |
| Free text, several lines | `TextAreaInput` | `CodeEditor` (unless it is code) |
| A secret | `PasswordInput` | `TextInput type` tricks |
| A search or filter term | `SearchInput` | `TextInput` with a magnifier icon |
| A number | `NumberInput` | `TextInput` (no clamping, no steppers) |
| A value to copy, not edit | `CopyInput` | a read-only `TextInput` + button |
| One of 2–5 short options, always visible, switching a view or mode | `ToggleInput` | `SelectInput`, `RadioGroup` |
| One of a few options with descriptions, in a form | `RadioGroup` | `ToggleInput` (no descriptions), `SelectInput` (hides choices) |
| One of many options, or options from the server | `SelectInput` | `RadioGroup` |
| Several of many options | `MultiSelectInput` | many checkboxes |
| Several of a few options, all visible | `CheckboxGroup` | `MultiSelectInput` |
| Free-form list of strings (emails, tags) | `TagInput` | `MultiSelectInput` with `onCreate` for a closed list |
| Yes/no confirmed on submit | `CheckboxInput` | `SwitchInput` |
| On/off that applies immediately | `SwitchInput` | `CheckboxInput` |
| A number in a range, dragged | `SliderInput` | `NumberInput` when the exact value matters |
| A date, date-time or range | `DateInput` | `TextInput` |
| A duration | `DurationInput` | `NumberInput` in seconds |
| A schedule | `CronInput` | a raw cron `TextInput` |
| A color | `ColorInput` | a hex `TextInput` |
| Files | `FileInput` | a native `<input type="file">` |
| Key/value pairs (headers, env vars) | `KeyValueInput` | rows of two `TextInput`s |
| A one-time code | `PinInput` | `TextInput` |

**Every field** takes `label` (rendered through `Field`), `error` (the app's message; also `aria-invalid` and the red frame), `isRequired`, `isDisabled`, `isReadOnly`, `size` (its own `<Input>Size`, default `MEDIUM`), `variant` (surface set, default `PRIMARY`), `isGhost` (no fill or hairline at rest: toolbars, table cells, inline editing), `fillWidth` and `ariaLabel` when there is no visible label. `onChange` receives the value. **No field validates**: the app computes `error`.

### Field

`@galaxy-io/dls/inputs/Field` · The frame around one control: `label`, `description`, `error`, `isRequired` / `isOptional`, `labelTooltip`, `actions` (a link at the end of the label row), `maxLength` + `count` (a counter), `orientation` (`HORIZONTAL` puts the label in a left column), `size`. Wires ids and `aria-describedby` to the control through `useField()`.

- Put the label on the Field or on the input, never both. Don't nest Fields; a group of controls is a `Fieldset`.

```tsx
import { useState } from "react";
import Field from "@galaxy-io/dls/inputs/Field";
import TextAreaInput from "@galaxy-io/dls/inputs/TextAreaInput";

export function DescriptionField() {
  const [value, setValue] = useState("");
  return (
    <Field
      label="Description"
      description="Shown to everyone in the workspace."
      maxLength={280}
      count={value.length}
      isOptional
      {...(value.length > 280 ? { error: "Keep it under 280 characters." } : {})}
    >
      <TextAreaInput value={value} onChange={setValue} fillWidth />
    </Field>
  );
}
```

### Fieldset

`@galaxy-io/dls/inputs/Fieldset` · Names a group of related controls: a `<fieldset>` with `label` (the legend), `description`, a group-level `error`, `isRequired` / `isOptional`, `size`. Children are usually `Field`s, laid out in a column.

```tsx
import Fieldset from "@galaxy-io/dls/inputs/Fieldset";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

<Fieldset label="Connection" description="Use a read-only user.">
  <TextInput label="Host" fillWidth />
  <TextInput label="Port" defaultValue="5432" />
</Fieldset>;
```

### Input

`@galaxy-io/dls/inputs/Input` · The single-line frame the text inputs share; it exports `InputSize` and `InputVariant`, which `TextInput`, `PasswordInput`, `SearchInput`, `NumberInput` and `CopyInput` use. Apps render a wrapper, not `Input`.

```tsx
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";

<TextInput label="Name" size={InputSize.SMALL} variant={InputVariant.SECONDARY} />;
```

### TextInput

`@galaxy-io/dls/inputs/TextInput` · Single-line text. `value` / `defaultValue` / `onChange`, `onEnter`, `placeholder` (never the label), `icon`, `leading` / `trailing`, `prefix` / `suffix` (text not part of the value), `isClearable`, `family` (mono for ids and hosts), `maxLength`, `autoComplete`, `inputMode`.

```tsx
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

<TextInput
  label="Webhook URL"
  prefix="https://"
  family={FontFamily.MONO}
  placeholder="hooks.acme.com/galaxy"
  isRequired
  fillWidth
/>;
```

### PasswordInput

`@galaxy-io/dls/inputs/PasswordInput` · Masked text with a "Show password" toggle. `autoComplete` defaults to `"current-password"` (use `"new-password"` on sign-up). Don't add your own eye button; don't mask a value meant to be copied (`CopyInput`).

```tsx
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

<PasswordInput label="Password" autoComplete="new-password" isRequired />;
```

### SearchInput

`@galaxy-io/dls/inputs/SearchInput` · Search or filter: a magnifier, a clear button (and Escape), `onSearch` after `debounceMs` (150) of quiet, at once on Enter and when emptied; `hotKeys` + `shouldBindHotKey` to focus it; `isLoading`. Named "Search" by default.

```tsx
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";

<SearchInput
  placeholder="Search sources"
  size={InputSize.SMALL}
  hotKeys={["/"]}
  shouldBindHotKey
  onSearch={(term) => console.log(term)}
/>;
```

### NumberInput

`@galaxy-io/dls/inputs/NumberInput` · One number (`number | null`). `onChange` fires on **commit** (blur, Enter, a step), clamped to `min` / `max` and rounded to `precision`; `step`, `hasSteppers`, `format` (display while blurred), units in `suffix` / `prefix`. Not for ZIP codes or account numbers (those are text).

```tsx
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";

<NumberInput label="Timeout" suffix="s" min={1} max={300} defaultValue={30} hasSteppers />;
```

### CopyInput

`@galaxy-io/dls/inputs/CopyInput` · A value the user copies but does not edit (an API key, a webhook URL), with an in-field copy button named from the label. `family={FontFamily.MONO}` for keys and hosts.

```tsx
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

<CopyInput label="Webhook URL" value="https://hooks.getgalaxy.io/w/8f2c" family={FontFamily.MONO} fillWidth />;
```

### TextAreaInput

`@galaxy-io/dls/inputs/TextAreaInput` · Multi-line text: `minRows`, `maxRows`, `isAutoGrow` or `isResizable` (not both), `isLoading` (skeleton), `family`. Count characters with `Field maxLength count`. Code belongs in `CodeEditor`.

```tsx
import TextAreaInput from "@galaxy-io/dls/inputs/TextAreaInput";

<TextAreaInput label="Notes" minRows={3} maxRows={8} isAutoGrow fillWidth />;
```

### SelectInput

`@galaxy-io/dls/inputs/SelectInput` · Pick one value from a list: `options` (`SelectOption`: `id`, `label`, `description`, `icon`, `leading`, `group`, `isDisabled` + `disabledReason`), `value` / `defaultValue` / `onChange(id | null)`, `isSearchable` (+ `onSearch` for server search), `isClearable`, `isLoading`, `onEndReached` (paging), `pinnedIds`, `renderOption`, `renderValue`. Long lists are virtualized.

- Not for commands (a `Menu`), not for two to four always-visible choices (`ToggleInput` or `RadioGroup`).

```tsx
import { useState } from "react";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";

export function RegionSelect() {
  const [region, setRegion] = useState<string | null>("us-east-1");
  return (
    <SelectInput
      label="Region"
      options={[
        { id: "us-east-1", label: "US East", description: "Virginia" },
        { id: "eu-west-1", label: "EU West", description: "Ireland" },
      ]}
      value={region}
      onChange={setRegion}
      isSearchable
    />
  );
}
```

### MultiSelectInput

`@galaxy-io/dls/inputs/MultiSelectInput` · Pick several values from a list, shown as chips: SelectInput's options and search, plus `maxSelected`, `onCreate` (with `isSearchable`) to add a missing option. `value` is an array of ids in selection order.

```tsx
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";

<MultiSelectInput
  label="Notify"
  options={[
    { id: "ada", label: "Ada Lovelace" },
    { id: "grace", label: "Grace Hopper" },
    { id: "alan", label: "Alan Turing" },
  ]}
  defaultValue={["ada"]}
  isSearchable
  fillWidth
/>;
```

### TagInput

`@galaxy-io/dls/inputs/TagInput` · A free-form list of strings typed as chips: Enter, `separators` (default `,`), paste and blur commit; `isTagInvalid` marks bad entries (the app's rule). `onChange` receives the whole list.

```tsx
import TagInput from "@galaxy-io/dls/inputs/TagInput";

<TagInput
  label="Recipients"
  placeholder="name@company.com"
  isTagInvalid={(tag) => !tag.includes("@")}
  fillWidth
/>;
```

### CheckboxInput

`@galaxy-io/dls/inputs/CheckboxInput` · One on/off choice confirmed later (on submit): `isChecked` / `defaultIsChecked` / `onChange(isChecked, event)`, `label`, `description`, `isIndeterminate` (a parent of a partly checked group; visual only).

```tsx
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";

<CheckboxInput label="Send me a weekly summary" description="Every Monday at 09:00." defaultIsChecked />;
```

### CheckboxGroup

`@galaxy-io/dls/inputs/CheckboxGroup` · One question, any number of answers, all visible: `options` (`id`, `label`, `description`, `isDisabled`), `value` (checked ids, in `options` order), `label`, `orientation`.

```tsx
import CheckboxGroup from "@galaxy-io/dls/inputs/CheckboxGroup";

<CheckboxGroup
  label="Events"
  options={[
    { id: "run.failed", label: "Run failed" },
    { id: "run.succeeded", label: "Run succeeded" },
    { id: "schema.changed", label: "Schema changed" },
  ]}
  defaultValue={["run.failed"]}
/>;
```

### RadioInput

`@galaxy-io/dls/inputs/RadioInput` · One option of a set where exactly one is chosen; controlled only (`isSelected` + `onChange`). A list of options is `RadioGroup`: use `RadioInput` only for a custom layout, with a shared `name`.

```tsx
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";

<RadioInput name="plan" label="Team" description="Up to 20 seats." isSelected onChange={() => {}} />;
```

### RadioGroup

`@galaxy-io/dls/inputs/RadioGroup` · One question, exactly one answer from a few visible options: `options` (`id`, `label`, `description`), `value` / `defaultValue` / `onChange(id)`, `label`, `orientation`. Preselect a sensible default.

```tsx
import RadioGroup from "@galaxy-io/dls/inputs/RadioGroup";

<RadioGroup
  label="Sync mode"
  options={[
    { id: "incremental", label: "Incremental", description: "Only rows that changed." },
    { id: "full", label: "Full refresh", description: "Every row, every run." },
  ]}
  defaultValue="incremental"
/>;
```

### SwitchInput

`@galaxy-io/dls/inputs/SwitchInput` · On/off for a setting that **applies immediately**: `isChecked` / `defaultIsChecked` / `onChange`, `label`, `description`, `isLabelTrailing`, `isLoading` (a change in flight), `fillWidth` (track at the row's end). Not in a form that saves on submit (use `CheckboxInput`).

```tsx
import SwitchInput from "@galaxy-io/dls/inputs/SwitchInput";

<SwitchInput label="Pause syncing" description="Runs resume where they stopped." fillWidth />;
```

### ToggleInput

`@galaxy-io/dls/inputs/ToggleInput` · The segmented control: one of two to five short options side by side (`options`: `id`, `label` and/or `icon`, `tooltip`, `isDisabled`), `value` / `defaultValue` / `onChange(id)`. It looks like an attached `ButtonGroup`: one `border.secondary` frame, one hairline at each seam, round outer corners only, the selected segment filled `background.selected`; `isGhost` drops the frame and seams. Default `variant` (the fill behind the unselected segments) is `SECONDARY`. Use it for a view mode or a short setting; not for on/off (`SwitchInput`), not for actions, not for six or more options.

```tsx
import { ListIcon, SquaresFourIcon } from "@phosphor-icons/react";
import ToggleInput, { ToggleInputSize } from "@galaxy-io/dls/inputs/ToggleInput";

<ToggleInput
  ariaLabel="Layout"
  size={ToggleInputSize.SMALL}
  options={[
    { id: "list", icon: ListIcon, ariaLabel: "List" },
    { id: "grid", icon: SquaresFourIcon, ariaLabel: "Grid" },
  ]}
  defaultValue="list"
/>;
```

### SliderInput

`@galaxy-io/dls/inputs/SliderInput` · A value (or with `isRange`, a `[low, high]` pair) on a rail: `min`, `max`, `step`, `marks`, `formatValue` (units, also `aria-valuetext`), `hasValueTooltip`, `onChange` (every move) and `onChangeEnd` (commit expensive work here).

```tsx
import SliderInput from "@galaxy-io/dls/inputs/SliderInput";

<SliderInput
  label="Sample rate"
  min={0}
  max={100}
  step={5}
  defaultValue={40}
  formatValue={(value) => `${value}%`}
  hasValueTooltip
/>;
```

### DateInput

`@galaxy-io/dls/inputs/DateInput` · A date, a date-time (`hasTime`) or a range (`isRange`, value `{ start, end }`): `min`, `max`, `timeZone`, `presets` ("Last 7 days"), `format`, `isClearable`.

```tsx
import DateInput from "@galaxy-io/dls/inputs/DateInput";

<DateInput
  label="Period"
  isRange
  presets={[
    { label: "Last 7 days", value: { start: new Date(Date.now() - 7 * 864e5), end: new Date() } },
  ]}
  onChange={(range) => console.log(range?.start, range?.end)}
/>;
```

### DurationInput

`@galaxy-io/dls/inputs/DurationInput` · A duration in hours, minutes and seconds; the value is milliseconds (`number | null`), committed on blur, Enter or a step, clamped to `min` / `max`.

```tsx
import DurationInput from "@galaxy-io/dls/inputs/DurationInput";

<DurationInput label="Max run time" defaultValue={90 * 60 * 1000} max={24 * 60 * 60 * 1000} />;
```

### CronInput

`@galaxy-io/dls/inputs/CronInput` · A five-field cron expression (`value` / `onChange`, `""` is empty) in a field that opens a schedule panel: `presets` (built-in list, or the app's), a frequency builder, the expression as text and a readable summary. `isClearable`, `isOpen` / `onOpenChange`.

```tsx
import CronInput from "@galaxy-io/dls/inputs/CronInput";

<CronInput
  label="Schedule"
  defaultValue="0 2 * * *"
  presets={[
    { label: "Nightly at 02:00", value: "0 2 * * *" },
    { label: "Weekdays at 06:00", value: "0 6 * * 1-5" },
  ]}
/>;
```

### ColorInput

`@galaxy-io/dls/inputs/ColorInput` · Pick a color (`#rrggbb` or `null`): a picker panel with `presets`, `onChange` on commit and `onPreviewChange` while dragging, `isClearable`. For user data (a label color), not for theming the UI.

```tsx
import ColorInput from "@galaxy-io/dls/inputs/ColorInput";

<ColorInput label="Label color" defaultValue="#7c3aed" presets={["#7c3aed", "#db2777", "#0891b2"]} />;
```

### FileInput

`@galaxy-io/dls/inputs/FileInput` · A drop zone plus a "Choose files" button and the chosen-file list: `accept`, `isMultiple`, `value` / `onChange(files)`, `onReject`, `renderFileActions` (upload progress or status per row). Uploading is the app's job.

```tsx
import FileInput from "@galaxy-io/dls/inputs/FileInput";

<FileInput label="Schema files" accept=".json,.yaml" isMultiple onChange={(files) => console.log(files)} />;
```

### KeyValueInput

`@galaxy-io/dls/inputs/KeyValueInput` · Editable rows of `{ key, value }` (HTTP headers, environment variables) with add and remove buttons; paste splits `KEY=value` lines.

```tsx
import KeyValueInput from "@galaxy-io/dls/inputs/KeyValueInput";

<KeyValueInput label="Headers" defaultValue={[{ key: "Authorization", value: "Bearer …" }]} fillWidth />;
```

### PinInput

`@galaxy-io/dls/inputs/PinInput` · A one-time code in `length` boxes: typing advances, paste fills, `onComplete` fires when full; `type` (`NUMERIC` / `ALPHANUMERIC`), `isMasked`.

```tsx
import PinInput from "@galaxy-io/dls/inputs/PinInput";

<PinInput label="Verification code" length={6} onComplete={(code) => console.log(code)} />;
```

### useField

`@galaxy-io/dls/hooks/useField` · For a custom control inside a `Field`: returns the `controlId`, `labelId`, `describedBy`, `isRequired` and `isError` the control must carry (`null` outside a Field).

```tsx
import { useField } from "@galaxy-io/dls/hooks/useField";

export function CustomControl() {
  const field = useField();
  return (
    <div
      role="group"
      id={field?.controlId}
      aria-labelledby={field?.labelId}
      aria-describedby={field?.describedBy}
      aria-invalid={field?.isError}
    />
  );
}
```

## Data display

| You need | Use | Not |
|---|---|---|
| A short tag, category, filter or count | `Chip` | `Beacon`, a colored `Text` |
| A set of chips that may overflow | `ChipGroup` | a `Flex` that wraps forever |
| The live state of one thing (Running, Failed) | `Beacon` (with `label`) | a status `Chip` when it is a filterable value |
| A person or entity | `Avatar` | an `Icon` |
| Several people | `AvatarGroup` | a row of `Avatar`s |
| A color key beside a label (legend, category) | `Circle` (lines, points) / `Square` (bars, areas) | `Beacon` (that is a state) |
| Labelled values of one object | `DescriptionList` | a two-column table |
| Events in time order | `Timeline` | a `BulletedList` with dates |
| A hierarchy the user browses or picks from | `Tree` | nested lists |
| A JSON value to explore | `JsonViewer` | a `CodeBlock` of `JSON.stringify` |

**Tree vs JsonViewer.** `Tree` is for your own hierarchical data (files, schemas, folders): labels, icons, selection, lazy children. `JsonViewer` is for an arbitrary parsed JSON value: typed coloring, search, copy path and value, no selection.

### Chip

`@galaxy-io/dls/chips/Chip` · A compact tinted label: `label`, `variant` (meaning, default `SECONDARY` neutral) or `color` (category), `icon` / `leading`, `hasDot`, `count` (+ `max`; a chip with only a count is a badge), `isPill`, `hasBorder`, `isSelected`, `onClick` (a filter chip, `aria-pressed`), `onDismiss` (a × button), `tooltip`. Chip rungs are 16 / 20 / 24 / 28 / 32px, sitting inside the control rung of the same name.

- Short labels. No hue per chip for decoration; no chip as an action; no interactive chip inside a `Button` or clickable row.

```tsx
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

<>
  <Chip label="Postgres" color="sky" />
  <Chip label="Failed" variant={ChipVariant.ERROR} hasDot size={ChipSize.SMALL} />
  <Chip count={12} variant={ChipVariant.PRIMARY} hasBorder isPill ariaLabel="12 unread" />
  <Chip label="owner:ada" onDismiss={() => {}} />
</>;
```

### ChipGroup

`@galaxy-io/dls/chips/ChipGroup` · One wrapping row of related chips with a fixed gap; `max` collapses the rest behind a `+N` chip that expands in place. Give each chip the `ChipSize` matching the group's `size`.

```tsx
import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import ChipGroup, { ChipGroupSize } from "@galaxy-io/dls/chips/ChipGroup";

<ChipGroup max={3} size={ChipGroupSize.SMALL} ariaLabel="Tags">
  {["finance", "daily", "pii", "warehouse", "beta"].map((tag) => (
    <Chip key={tag} label={tag} size={ChipSize.SMALL} />
  ))}
</ChipGroup>;
```

### Beacon

`@galaxy-io/dls/beacons/Beacon` · A status dot that can pulse: `variant` (`PRIMARY` default, statuses) or `color`, `label` (the whole status, "Running"), `isPulse` (the one live row), `size` (the rung of the text beside it). A dot alone needs `ariaLabel` or adjacent text.

```tsx
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";

<>
  <Beacon label="Running" isPulse />
  <Beacon label="Failed" variant={BeaconVariant.ERROR} />
</>;
```

### Avatar

`@galaxy-io/dls/avatar/Avatar` · One person or entity: `img`, `name` (initials fallback and accessible name), `seed` (a generated dot pattern), `size` (a rung or px), `isSquare` (entities: workspaces, projects, sources), `color` (tints the fallback), `isLoading`.

```tsx
import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";

<>
  <Avatar name="Ada Lovelace" size={AvatarSize.SMALL} />
  <Avatar name="Acme Analytics" seed="ws_8f2c" isSquare />
</>;
```

### AvatarGroup

`@galaxy-io/dls/avatar/AvatarGroup` · An overlapping row of `Avatar`s: `max` (a `+N` tile lists the rest in a tooltip), `size`, `isSquare` for every child. One kind per group.

```tsx
import Avatar from "@galaxy-io/dls/avatar/Avatar";
import AvatarGroup, { AvatarGroupSize } from "@galaxy-io/dls/avatar/AvatarGroup";

<AvatarGroup max={3} size={AvatarGroupSize.SMALL} ariaLabel="Pipeline owners">
  <Avatar name="Ada Lovelace" />
  <Avatar name="Grace Hopper" />
  <Avatar name="Alan Turing" />
  <Avatar name="Katherine Johnson" />
</AvatarGroup>;
```

### Circle and Square

`@galaxy-io/dls/shapes/Circle` and `@galaxy-io/dls/shapes/Square` · Small filled marks beside a label: a legend key or a category key. `variant` (default `TERTIARY`) or `color`, `size` (the rung of the text beside it), `hex` for data colors. A circle keys a line or point series; a square keys a bar, area or heatmap. Pair with text; a bare mark is not a signal.

```tsx
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Circle from "@galaxy-io/dls/shapes/Circle";
import Square from "@galaxy-io/dls/shapes/Square";
import Text from "@galaxy-io/dls/text/Text";

<Flex gap={16} alignItems={AlignItems.CENTER}>
  <Flex gap={4} alignItems={AlignItems.CENTER}>
    <Circle color="purple" />
    <Text>Signups</Text>
  </Flex>
  <Flex gap={4} alignItems={AlignItems.CENTER}>
    <Square color="teal" />
    <Text>Revenue</Text>
  </Flex>
</Flex>;
```

### DescriptionList

`@galaxy-io/dls/lists/DescriptionList` · Labelled values in a `<dl>`: `items` (`label`, `value` string / number / node, `family` for mono values, `isCopyable` + `copyValue`), `orientation` (`HORIZONTAL` aligns labels in a column), `columns` (1–4), `size`. Empty values render an em dash.

```tsx
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import DescriptionList from "@galaxy-io/dls/lists/DescriptionList";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

<DescriptionList
  items={[
    { label: "Status", value: <Beacon label="Healthy" variant={BeaconVariant.SUCCESS} /> },
    { label: "Source id", value: "src_8f2c91", family: FontFamily.MONO, isCopyable: true },
    { label: "Owner", value: "Ada Lovelace" },
    { label: "Last error", value: null },
  ]}
/>;
```

### Timeline

`@galaxy-io/dls/timeline/Timeline` · Events in order on a hairline rail (a run's history, an audit trail, an agent's steps): `items` (`id`, `label`, `time` + `dateTime`, `description`, `footer`, `icon`, `isActive`, `variant` or `color`), `size`, `markerShape` (`SQUARE` box by default, `CIRCLE`, or `NONE`: no box, the rail leads to each dot or icon and stops 4px short; use `NONE` for a long, quiet feed), `isFilled` (fill each box with its item's background role, as a `Chip` fills). Not interactive.

```tsx
import { CheckIcon, WarningIcon } from "@phosphor-icons/react";
import Timeline, { TimelineVariant } from "@galaxy-io/dls/timeline/Timeline";

<Timeline
  items={[
    { id: "1", label: "Run started", time: "14:02", dateTime: "2026-10-05T14:02:00Z" },
    { id: "2", label: "Schema changed", time: "14:03", icon: WarningIcon, variant: TimelineVariant.WARNING },
    { id: "3", label: "Run finished", time: "14:09", icon: CheckIcon, variant: TimelineVariant.SUCCESS },
  ]}
/>;
```

### Tree

`@galaxy-io/dls/tree/Tree` · Hierarchical rows that expand and collapse: `items` (`id`, `label`, `icon`, `trailing`, `children`, `hasChildren` for lazy loading, `isDisabled`), one selected row (`value` / `onChange`), `expandedIds` / `onExpandedIdsChange`, `size`. Requires `ariaLabel` (or `aria-labelledby`).

```tsx
import { FileIcon, FolderIcon } from "@phosphor-icons/react";
import Tree from "@galaxy-io/dls/tree/Tree";

<Tree
  ariaLabel="Models"
  defaultExpandedIds={["marts"]}
  items={[
    {
      id: "marts",
      label: "marts",
      icon: FolderIcon,
      children: [{ id: "orders.sql", label: "orders.sql", icon: FileIcon }],
    },
  ]}
  onChange={(id) => console.log(id)}
/>;
```

### JsonViewer

`@galaxy-io/dls/json/JsonViewer` · A parsed JSON value as an explorable tree: `data`, `defaultExpandedDepth`, `canSearch`, `canCopy` (path and value), `size`. Requires `ariaLabel`.

```tsx
import JsonViewer from "@galaxy-io/dls/json/JsonViewer";

<JsonViewer
  ariaLabel="Event payload"
  data={{ id: "evt_1", type: "run.failed", attempts: 3, tags: ["nightly"], error: null }}
  canSearch
  canCopy
/>;
```

## Feedback

| You need to say | Use | Not |
|---|---|---|
| Something about what is on screen, lasting until resolved (a failed sync, a degraded source) | `Alert` (inline or `isBanner`) | a `Toast` |
| The result of something the user just did (Saved, Copied, Deleted with Undo) | `Toast` via `useToast()` | an `Alert` |
| The live state of one item | `Beacon` | a `Toast` |
| A short status value on a row (Failed, Beta) | `Chip` | an `Alert` |
| There is nothing here yet | `EmptyState` | blank space, or an `Alert` |

| Loading | Use |
|---|---|
| The shape of the content is known (rows, a card, text lines) | `Skeleton` |
| Something is working, duration unknown, small space (a button, a row, a panel) | `Spinner` |
| Progress with a known total, wide | `ProgressBar` |
| Progress with a known total, icon-sized (a row, a file) | `ProgressCircle` |
| A value that exists and is updating | `Flasher` |
| Text being generated | `Ellipsis` |

### Alert

`@galaxy-io/dls/feedback/Alert` · A message about the thing on screen: `variant` (`PRIMARY SUCCESS WARNING ERROR`), `header`, the body (`children`), `actions` (usually one `ButtonSize.SMALL` button), `onDismiss`, `icon`, `isBanner` (a full-width strip). `ERROR` is `role="alert"`.

- Give an `ERROR` a way out (a retry, a link to fix it). One Alert per problem.
- The way out is one `PRIMARY` button at `ButtonSize.SMALL`; a second or informational action ("View logs", "Details") is `TERTIARY`. Never `SECONDARY`: its framed surface reads as a hole in the tinted alert.

```tsx
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

<Alert
  variant={AlertVariant.ERROR}
  header="Sync failed"
  actions={<Button label="Retry" size={ButtonSize.SMALL} variant={ButtonVariant.PRIMARY} />}
>
  The orders table is locked by another process.
</Alert>;
```

### Toast and useToast

`@galaxy-io/dls/toast/useToast` · `useToast()` returns `{ toast, dismiss }` anywhere under `GalaxyProvider`. `toast({ header, description, variant, action, isLoading, duration, isPersistent, placement, id, onDismiss })` returns an id; `toast.promise(promise, { loading, success, error })` follows a promise. `@galaxy-io/dls/toast/Toast` is the card (and exports `ToastVariant`); `@galaxy-io/dls/toast/ToastProvider` and `@galaxy-io/dls/toast/ToastViewport` are mounted by `GalaxyProvider` and only needed in a React root without it.

- One toast per event; one short line; an action only when it can be undone or followed ("Undo", "View"). Never the only place an outcome is shown.

```tsx
import Button from "@galaxy-io/dls/buttons/Button";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

declare function archiveSource(): Promise<void>;

export function ArchiveButton() {
  const { toast } = useToast();
  return (
    <Button
      label="Archive"
      onClick={() => {
        toast.promise(archiveSource(), {
          loading: "Archiving source…",
          success: { header: "Source archived", variant: ToastVariant.SUCCESS, action: { label: "Undo", onClick: () => {} } },
          error: "Could not archive the source",
        });
      }}
    />
  );
}
```

### EmptyState

`@galaxy-io/dls/feedback/EmptyState` · What a region shows when it has nothing: `header` (what would be here), `description` (why, or what to do), `icon`, `actions` (one `PRIMARY` and one `SECONDARY` button), `size` (`SMALL` cell or chart, `MEDIUM` panel, `LARGE` page). Pass `role="status"` when it replaces results after a search or filter. A failure is an `Alert`, not an EmptyState.

```tsx
import { PlugsIcon } from "@phosphor-icons/react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import EmptyState, { EmptyStateSize } from "@galaxy-io/dls/feedback/EmptyState";

<EmptyState
  size={EmptyStateSize.LARGE}
  icon={PlugsIcon}
  header="No sources yet"
  description="Connect a database or an app to start syncing data."
  actions={
    <>
      <Button label="Connect source" />
      <Button label="Read the guide" variant={ButtonVariant.SECONDARY} />
    </>
  }
/>;
```

### Spinner

`@galaxy-io/dls/feedback/Spinner` · The indeterminate loading indicator, a polite live region: `size` (the rung it sits in), `variant` (`INHERIT` inside hosts), `label` (visible text) or `ariaLabel` (default "Loading"), `children` (turn your own glyph). Buttons, selects and tables already show their own.

```tsx
import Spinner from "@galaxy-io/dls/feedback/Spinner";

<Spinner label="Loading runs…" />;
```

### ProgressBar

`@galaxy-io/dls/feedback/ProgressBar` · A horizontal bar: `value` (0–100, clamped) or `isIndeterminate`, `label` or `ariaLabel` (one is required), `shouldShowValue`, `variant` (neutral or a status) or `color`, `size` (track thickness and text step). Spans its parent.

```tsx
import ProgressBar from "@galaxy-io/dls/feedback/ProgressBar";

<ProgressBar label="Syncing contacts" value={42} shouldShowValue />;
```

### ProgressCircle

`@galaxy-io/dls/feedback/ProgressCircle` · The ring sibling of `ProgressBar` with the same props (no `isIndeterminate`), at the icon size of its rung, so it swaps in for a `Spinner` once the total is known.

```tsx
import ProgressCircle from "@galaxy-io/dls/feedback/ProgressCircle";

<ProgressCircle value={75} label="Uploading" shouldShowValue />;
```

### Skeleton

`@galaxy-io/dls/feedback/Skeleton` · A pulsing placeholder in the shape of the content: `variant` (`TEXT` lines, `CIRCLE`, `RECT`), `size`, `lines`. Wrapper mode, `<Skeleton isLoading>{content}</Skeleton>`, paints over exactly the content's box. Always `aria-hidden`: set `aria-busy` on the loading region.

```tsx
import Skeleton, { SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";

<Flex direction={FlexDirection.COLUMN} gap={8} aria-busy>
  <Skeleton variant={SkeletonVariant.CIRCLE} />
  <Skeleton lines={3} />
</Flex>;
```

## Overlays

| You need | Use | Not |
|---|---|---|
| A short, non-interactive hint on hover or focus | `Tooltip` | a `Popover` |
| Interactive content anchored to a trigger (a form, a picker, a preview on hover) | `Popover` | a `Tooltip` (it cannot hold controls) |
| A click-opened panel with header, body and footer chrome (filters with Apply) | `Dropdown` | a `Menu` |
| A list of commands from a button | `Menu` | a `Dropdown` with buttons in it |
| The same commands on right-click | `ContextMenu` (plus another way to reach them) | a context menu as the only way |
| A blocking task with actions (create, edit) | `Modal` | a `Drawer` |
| A yes/no question before an action | `ConfirmDialog` | a hand-built `Modal` |
| Detail beside a list, or a side task that keeps the page visible | `Drawer` | a `Modal` |
| App-wide "type to do anything" | `CommandPalette` | a `Menu` with search |

Every overlay takes `isOpen` / `defaultIsOpen` / `onOpenChange`, renders through `Portal`, joins the overlay stack (Escape closes the topmost only) and restores focus on close. Keep Modal, Drawer and ConfirmDialog mounted and drive them with `isOpen`, so they can animate out.

### Tooltip

`@galaxy-io/dls/tooltip/Tooltip` · A hint beside one trigger on hover or keyboard focus: `body` (a string renders `BODY_SM`), `hotKeys`, `placement`, `delay`, `variant` (fill inside the always-inverted bubble). The trigger is the single child (it must forward its ref). Never put essential information or controls in a tooltip. Many components take a `tooltip` prop directly.

```tsx
import { GearIcon } from "@phosphor-icons/react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

<Tooltip body="Settings" hotKeys={["mod", ","]}>
  <Button icon={GearIcon} ariaLabel="Settings" variant={ButtonVariant.TERTIARY} />
</Tooltip>;
```

### Popover

`@galaxy-io/dls/overlays/Popover` · The anchored panel: `body`, `trigger` (`PopoverTrigger.CLICK HOVER FOCUS CONTEXT_MENU MANUAL`), `placement`, `variant`, `hasSurface`, `role` (`PopoverRole`), `shouldMatchTriggerWidth`, `delay`. Closes on Escape, an outside press or focus leaving.

```tsx
import Avatar from "@galaxy-io/dls/avatar/Avatar";
import Popover, { PopoverTrigger } from "@galaxy-io/dls/overlays/Popover";

<Popover trigger={PopoverTrigger.HOVER} delay={300} body="Ada Lovelace · Owner · ada@acme.com">
  <Avatar name="Ada Lovelace" />
</Popover>;
```

### Dropdown

`@galaxy-io/dls/dropdown/Dropdown` · A click-opened panel with chrome: pinned `header`, scrolling `body`, pinned `footer` (each may be a function of `{ close }`), `placement`, `variant`, `hasSurface`, `shouldMatchTriggerWidth`. Also the `dropdown` of a split `Button`.

```tsx
import { FunnelIcon } from "@phosphor-icons/react";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Dropdown from "@galaxy-io/dls/dropdown/Dropdown";
import CheckboxGroup from "@galaxy-io/dls/inputs/CheckboxGroup";
import Flex, { JustifyContent } from "@galaxy-io/dls/layout/Flex";

<Dropdown
  header="Filter runs"
  body={
    <CheckboxGroup
      ariaLabel="Status"
      options={[
        { id: "failed", label: "Failed" },
        { id: "succeeded", label: "Succeeded" },
      ]}
    />
  }
  footer={({ close }) => (
    <Flex justifyContent={JustifyContent.END} gap={8} fillWidth>
      <Button label="Apply" size={ButtonSize.SMALL} onClick={close} />
    </Flex>
  )}
>
  <Button label="Filter" icon={FunnelIcon} variant={ButtonVariant.SECONDARY} size={ButtonSize.SMALL} />
</Dropdown>;
```

### Menu

`@galaxy-io/dls/menu/Menu` · A trigger that opens a list of commands: `trigger`, rows as children, `size` (`SMALL` 24px / `MEDIUM` 32px rows), `placement`, `isSearchable`. Rows, from the same module: `MenuItem` (`label`, `description`, `icon`, `hotKeys`, `onSelect` or `href`, `variant={MenuItemVariant.ERROR}` for destructive rows, `isDisabled` + `disabledReason`, or `children` for a submenu), `MenuCheckboxItem`, `MenuRadioGroup`, `MenuGroup` (a labelled section) and `MenuSeparator`.

- Commands only: no fields, no form values (that is `SelectInput`). Color only on `ERROR` rows.

```tsx
import { CopyIcon, DotsThreeIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Menu, { MenuItem, MenuItemVariant, MenuSeparator } from "@galaxy-io/dls/menu/Menu";

<Menu
  trigger={
    <Button icon={DotsThreeIcon} ariaLabel="Source actions" variant={ButtonVariant.TERTIARY} size={ButtonSize.SMALL} />
  }
>
  <MenuItem label="Rename" icon={PencilSimpleIcon} onSelect={() => {}} />
  <MenuItem label="Duplicate" icon={CopyIcon} hotKeys={["mod", "d"]} onSelect={() => {}} />
  <MenuSeparator />
  <MenuItem label="Delete" icon={TrashIcon} variant={MenuItemVariant.ERROR} onSelect={() => {}} />
</Menu>;
```

### ContextMenu

`@galaxy-io/dls/menu/ContextMenu` · A `Menu` opened by right-click (or Shift+F10) on a region, at the pointer. Same rows, imported from `menu/Menu`; `ariaLabel` is required. Every command must also be reachable another way.

```tsx
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import ContextMenu from "@galaxy-io/dls/menu/ContextMenu";
import { MenuItem } from "@galaxy-io/dls/menu/Menu";
import Text from "@galaxy-io/dls/text/Text";

<ContextMenu
  ariaLabel="File actions"
  trigger={
    <Box variant={BoxVariant.SECONDARY} padding={16}>
      <Text>orders.sql</Text>
    </Box>
  }
>
  <MenuItem label="Open" onSelect={() => {}} />
  <MenuItem label="Rename" onSelect={() => {}} />
</ContextMenu>;
```

### Modal

`@galaxy-io/dls/modal/Modal` · The blocking dialog: `header` (a string names it), `subheader` (a second line that describes it), `icon` (leading), the scrolling body, `footer` (actions, least destructive first), `size` (`SMALL` 400 / `MEDIUM` 560 / `LARGE` 720 / `X_LARGE` near full), `isDismissable`. No trigger: the app opens it with `isOpen` / `onOpenChange`.

```tsx
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useDisclosure } from "@galaxy-io/dls/hooks/useDisclosure";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";

export function RenameDialog() {
  const { isOpen, open, close, setIsOpen } = useDisclosure();
  return (
    <>
      <Button label="Rename" variant={ButtonVariant.SECONDARY} onClick={open} />
      <Modal
        header="Rename pipeline"
        size={ModalSize.SMALL}
        isOpen={isOpen}
        onOpenChange={setIsOpen}
        footer={
          <>
            <Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={close} />
            <Button label="Save" onClick={close} />
          </>
        }
      >
        <TextInput label="Name" defaultValue="Nightly orders sync" fillWidth autoFocus />
      </Modal>
    </>
  );
}
```

### ConfirmDialog

`@galaxy-io/dls/modal/ConfirmDialog` · One question before an action: `header` (the question), `description` (what happens), `label` (names the action, never "OK"), `onConfirm` (may return a promise: spinner while pending, closes on resolve, stays open on reject), `isDestructive` (`ERROR` button), `confirmValue` (type-to-confirm), `error`.

```tsx
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useDisclosure } from "@galaxy-io/dls/hooks/useDisclosure";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";

declare function deleteSource(): Promise<void>;

export function DeleteSource() {
  const { isOpen, open, setIsOpen } = useDisclosure();
  return (
    <>
      <Button label="Delete source" variant={ButtonVariant.ERROR} onClick={open} />
      <ConfirmDialog
        isOpen={isOpen}
        onOpenChange={setIsOpen}
        header="Delete orders_db?"
        description="Its 12 tables stop syncing. Data already in the warehouse stays."
        label="Delete source"
        isDestructive
        confirmValue="orders_db"
        onConfirm={deleteSource}
      />
    </>
  );
}
```

### Drawer

`@galaxy-io/dls/drawer/Drawer` · A panel from an edge that keeps the page visible: `header`, `subheader` (a second line that describes it), `icon` (leading), `actions` (title-row controls before the close button: one or two icon-only `TERTIARY` `SMALL` buttons and a ⋯ `Menu`, for the thing the drawer shows), body, `footer` (only what finishes the task: Cancel, then the `PRIMARY` action), `side` (`Side`, default `RIGHT`), `size`, `isResizable`, `isModal` (default `true`; `false` docks it beside the content with no backdrop), `isDismissable`.

```tsx
import Drawer, { DrawerSize } from "@galaxy-io/dls/drawer/Drawer";
import DescriptionList from "@galaxy-io/dls/lists/DescriptionList";

<Drawer
  header="Run 4812"
  subheader="orders_daily"
  size={DrawerSize.MEDIUM}
  isOpen
  onOpenChange={() => {}}
>
  <DescriptionList items={[{ label: "Duration", value: "6m 12s" }]} />
</Drawer>;
```

### CommandPalette

`@galaxy-io/dls/navigation/CommandPalette` · The ⌘K dialog: `items` (`id`, `label`, `description`, `icon`, `group`, `keywords`, `hotKeys`, `onSelect`), built-in filtering or `onSearch` + `isLoading` for server search, `storageKey` (recents), `shouldBindHotKey` (binds `hotKeys`, default `["mod", "k"]`).

```tsx
import { GearIcon, HouseIcon, PlusIcon } from "@phosphor-icons/react";
import CommandPalette from "@galaxy-io/dls/navigation/CommandPalette";

<CommandPalette
  shouldBindHotKey
  storageKey="galaxy:commands"
  items={[
    { id: "new-source", label: "New source", icon: PlusIcon, group: "Create", onSelect: () => {} },
    { id: "home", label: "Go to overview", icon: HouseIcon, group: "Navigate", onSelect: () => {} },
    { id: "settings", label: "Settings", icon: GearIcon, group: "Navigate", keywords: ["preferences"], hotKeys: ["mod", ","], onSelect: () => {} },
  ]}
/>;
```

### OverlayProvider and useOverlay

`@galaxy-io/dls/overlay/OverlayProvider` holds the overlay stack (mounted by `GalaxyProvider`; it also exports `OverlayNode`). `@galaxy-io/dls/overlay/useOverlay` registers a custom overlay: `useOverlay({ isOpen, layer: OverlayLayer.DROPDOWN, onEscape })` returns `zIndex`, `isTopmost` and `overlayId` (wrap the content in `<OverlayNode overlayId={…}>`). Only for overlays the DLS does not provide.

```tsx
import type { ReactNode } from "react";
import Portal from "@galaxy-io/dls/layout/Portal";
import { OverlayNode } from "@galaxy-io/dls/overlay/OverlayProvider";
import { OverlayLayer, useOverlay } from "@galaxy-io/dls/overlay/useOverlay";

export function Callout({ isOpen, onClose, children }: { isOpen: boolean; onClose: () => void; children: ReactNode }) {
  const { zIndex, overlayId } = useOverlay({ isOpen, layer: OverlayLayer.DROPDOWN, onEscape: onClose });
  if (!isOpen) return null;
  return (
    <Portal>
      <OverlayNode overlayId={overlayId}>
        <div style={{ "--callout-z": zIndex } as React.CSSProperties}>{children}</div>
      </OverlayNode>
    </Portal>
  );
}
```

## Navigation

| You need | Use |
|---|---|
| The app's primary destinations | `SidebarNav` with `NavItem` / `NavGroup` |
| The bar across the top of a page (title, breadcrumbs, app-wide actions) | `Topbar` |
| Peer views of one object or place | `Tabs` (`UNDERLINE` page level, `PILL` second level) |
| Where the page sits in a hierarchy | `Breadcrumbs` |
| Pages of a long list or table | `Pagination` (or infinite loading in the table) |
| Progress through a known sequence (a setup flow) | `Stepper` |
| A link in text | `Link` |

### SidebarNav

`@galaxy-io/dls/navigation/SidebarNav` · The primary navigation column: a 48px `header` (brand or workspace switcher), `NavItem`s and `NavGroup`s, a pinned `footer`, `isCollapsed` (a 48px icon rail). `NavItem`: `label`, `icon`, `href` (+ `as` for your router) or `onClick`, `isActive` (from the route), `count`, `trailing`, `tooltip`. `NavGroup`: `label`, `isCollapsible`. It fills its parent; the app owns the width and the collapsed state.

```tsx
import { DatabaseIcon, FlowArrowIcon, GearIcon, HouseIcon } from "@phosphor-icons/react";
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import Box from "@galaxy-io/dls/layout/Box";
import SidebarNav, { NavGroup, NavItem } from "@galaxy-io/dls/navigation/SidebarNav";

<Box width={240} height="100%">
  <SidebarNav
    header={<GalaxyLogomark ariaLabel="Galaxy" />}
    footer={<NavItem label="Settings" icon={GearIcon} href="/settings" />}
  >
    <NavItem label="Overview" icon={HouseIcon} href="/" isActive />
    <NavGroup label="Data">
      <NavItem label="Sources" icon={DatabaseIcon} href="/sources" count={12} />
      <NavItem label="Pipelines" icon={FlowArrowIcon} href="/pipelines" />
    </NavGroup>
  </SidebarNav>
</Box>;
```

### Topbar

`@galaxy-io/dls/navigation/Topbar` · The 48px page bar: `leading` (a sidebar toggle, a back button), the title or `Breadcrumbs` as children (truncates), `actions` (search, buttons, the account). Use `SMALL` controls and app-wide actions only; it does not stick or route.

```tsx
import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import Topbar from "@galaxy-io/dls/navigation/Topbar";

<Topbar
  actions={
    <>
      <SearchInput size={InputSize.SMALL} />
      <Avatar name="Ada Lovelace" size={AvatarSize.SMALL} />
    </>
  }
>
  Sources
</Topbar>;
```

### Tabs

`@galaxy-io/dls/navigation/Tabs` · Peer views: `items` (panel tabs `{ id, label, icon, count, panel }` or link tabs `{ id, label, href }` with `as`), `value` / `defaultValue` / `onChange`, `variant` (`UNDERLINE` / `PILL`), `size`, `orientation`, `fillWidth`, `actions`. Tabs that do not fit collapse into a "More" menu. Pass `ariaLabel` unless a visible heading names the tabs.

- Not for a view mode of the same data (that is `ToggleInput`); never two `UNDERLINE` rows.
- A horizontal row centers on the height it gets: put it straight into a header row that stretches its children (the default `align-items`) and it fills the row, its labels line up with the row's other centered content and the `UNDERLINE` indicator sits on the row's bottom edge; in an `align-items: center` row it keeps its own height.

```tsx
import Tabs from "@galaxy-io/dls/navigation/Tabs";
import Text from "@galaxy-io/dls/text/Text";

<Tabs
  ariaLabel="Source"
  items={[
    { id: "overview", label: "Overview", panel: <Text>Overview</Text> },
    { id: "runs", label: "Runs", count: 3, panel: <Text>Runs</Text> },
    { id: "settings", label: "Settings", panel: <Text>Settings</Text> },
  ]}
/>;
```

### Breadcrumbs

`@galaxy-io/dls/navigation/Breadcrumbs` · The trail from the top level down: `items` (`label`, `href`, `icon`; the last is the current page, as text), `separator`, `maxItems` (the middle collapses into a "…" menu), `as` (router link), `size`.

```tsx
import Breadcrumbs from "@galaxy-io/dls/navigation/Breadcrumbs";

<Breadcrumbs
  items={[
    { label: "Acme", href: "/" },
    { label: "Sources", href: "/sources" },
    { label: "orders_db" },
  ]}
/>;
```

### Pagination

`@galaxy-io/dls/navigation/Pagination` · Previous / next around a seven-slot page window: `total`, `pageSize`, `value` / `onChange` (1-based), `hasSummary` ("1–25 of 312"), `pageSizeOptions` + `onPageSizeChange`, `isCompact`, `size`. Reset to page 1 when filters change.

```tsx
import { useState } from "react";
import Pagination from "@galaxy-io/dls/navigation/Pagination";

export function RunsPagination({ total }: { total: number }) {
  const [page, setPage] = useState(1);
  return <Pagination total={total} pageSize={25} value={page} onChange={setPage} hasSummary fillWidth />;
}
```

### Stepper

`@galaxy-io/dls/navigation/Stepper` · Progress through a known, ordered sequence: `steps` (`label`, `description`, `isError`), `value` (zero-based current step; `steps.length` = all done), `onChange` (done steps become buttons to go back), `orientation`, `size`. The app moves forward.

```tsx
import Stepper from "@galaxy-io/dls/navigation/Stepper";

<Stepper
  value={1}
  steps={[{ label: "Source" }, { label: "Schema", description: "Pick tables" }, { label: "Schedule" }]}
/>;
```

## Tables

| You have | Use |
|---|---|
| Read-only rows you hold in memory (hundreds to a few thousand), with selection, expansion or client sorting | `InfiniteTable` |
| Very long lists (tens of thousands of rows) fetched by index; fixed row height; server sorting | `VirtualizedInfiniteTable` |
| Cells the user edits in place | `InfiniteSpreadsheet` |
| Labelled values of one record | `DescriptionList` (not a two-column table) |

All three share `TableColumn` from `@galaxy-io/dls/table/types` (`id`, `header`, `accessor`, `cell`, `width` / `minWidth` / `maxWidth`, `align`, `pin`, `isRowHeader`, `canSort`, `canResize`, `canHide`, `footer`, header `actions`), `size` (`SMALL MEDIUM LARGE`: 32 / 48 / 52px rows), `isLoading` (skeleton rows), `error`, `emptyState`, `onEndReached` (load more), `rowActions` (Menu rows behind a "…" button), `columnLayout` (order, hidden ids, widths; persist it) and `canCustomizeColumns`. **Bound the parent's height**: the table scrolls inside it.

### InfiniteTable

`@galaxy-io/dls/table/InfiniteTable` · `columns`, `data`, `getRowId` (always pass it when rows can move), `isSelectable` + `value` / `onChange` (selected ids), `sort` / `onSortChange` (client sorting, server sorting with `onEndReached`), `renderExpandedRow`, `onRowClick` + `activeRowId` (open a row in a side panel), `canResizeColumns`.

```tsx
import { MenuItem, MenuItemVariant } from "@galaxy-io/dls/menu/Menu";
import Box from "@galaxy-io/dls/layout/Box";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";

interface Source {
  id: string;
  name: string;
  rows: number;
}

const COLUMNS: TableColumn<Source>[] = [
  { id: "name", header: "Name", accessor: (row) => row.name, isRowHeader: true, canSort: true } satisfies TableColumn<Source, string>,
  { id: "rows", header: "Rows", accessor: (row) => row.rows, align: "right", canSort: true } satisfies TableColumn<Source, number>,
];

export const SourcesTable = ({ sources }: { sources: Source[] }) => (
  <Box height={480}>
    <InfiniteTable
      ariaLabel="Sources"
      columns={COLUMNS}
      data={sources}
      getRowId={(row) => row.id}
      isSelectable
      rowActions={() => <MenuItem label="Delete" variant={MenuItemVariant.ERROR} onSelect={() => {}} />}
    />
  </Box>
);
```

### VirtualizedInfiniteTable

`@galaxy-io/dls/table/VirtualizedInfiniteTable` · `InfiniteTable` for very long lists: `totalRowCount` and `getRowData(index)` (`undefined` renders a skeleton row) instead of `data`; only the rows in view are in the DOM. No selection, no expansion, no client sorting.

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import VirtualizedInfiniteTable from "@galaxy-io/dls/table/VirtualizedInfiniteTable";

interface Event {
  id: string;
  type: string;
}

declare const pages: Map<number, Event>;

const COLUMNS: TableColumn<Event>[] = [
  { id: "type", header: "Event", accessor: (row) => row.type, isRowHeader: true } satisfies TableColumn<Event, string>,
];

<Box height={600}>
  <VirtualizedInfiniteTable
    ariaLabel="Events"
    columns={COLUMNS}
    totalRowCount={250_000}
    getRowData={(index) => pages.get(index)}
    getRowId={(row) => row.id}
    onEndReached={() => {}}
  />
</Box>;
```

### InfiniteSpreadsheet

`@galaxy-io/dls/table/InfiniteSpreadsheet` · The editable grid: columns declare a `cellType` (`SpreadsheetCellType.TEXT NUMBER BOOLEAN ENUM DATE`, which picks the editor), an `accessor` and `onCellChange(value, row, rowIndex)` (the app stores it and validates); `getCellStatus` draws a status dot; `onAddRow`, `onAddColumn`. One roving cell; Enter, F2 or typing edits.

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import InfiniteSpreadsheet, {
  type InfiniteSpreadsheetColumn,
  SpreadsheetCellType,
} from "@galaxy-io/dls/table/InfiniteSpreadsheet";

interface Lead {
  company: string;
  seats: number | null;
}

declare const leads: Lead[];

const COLUMNS: InfiniteSpreadsheetColumn<Lead>[] = [
  {
    id: "company",
    header: "Company",
    cellType: SpreadsheetCellType.TEXT,
    accessor: (row) => row.company,
    onCellChange: (value, _row, index) => console.log(index, value),
  },
  {
    id: "seats",
    header: "Seats",
    cellType: SpreadsheetCellType.NUMBER,
    accessor: (row) => row.seats,
    onCellChange: (value, _row, index) => console.log(index, value),
  },
];

<Box height={400}>
  <InfiniteSpreadsheet
    ariaLabel="Leads"
    columns={COLUMNS}
    totalRowCount={leads.length}
    getRowData={(index) => leads[index]}
  />
</Box>;
```

### Table building blocks

For tables the three components do not cover. Prefer the components; these change more often.

- `@galaxy-io/dls/table/types`: `TableColumn`, `TableSort`, `TableColumnLayout`, `TableCellContext`.
- `@galaxy-io/dls/table/TableCore`: the shared frame, header cell, rows, skeleton row and metrics.
- `@galaxy-io/dls/table/SpreadsheetCellEditor`: the bare cell editors of `InfiniteSpreadsheet`.
- Hooks: `@galaxy-io/dls/table/useTableColumnLayout` (order, hidden, widths), `@galaxy-io/dls/table/useTableRowSelection`, `@galaxy-io/dls/table/useTableKeyboardNavigation`, `@galaxy-io/dls/table/useSpreadsheetCellNavigation`, `@galaxy-io/dls/table/useTableVirtualizer`, `@galaxy-io/dls/table/useInfiniteScrollSentinel`.

## Charts

| You want to show | Use | Not |
|---|---|---|
| A trend over ordered categories (time) | `LineChart` | a `BarChart` with many categories |
| Volume over time, or parts of a total over time | `AreaChart` (`isStacked` for parts) | more than three overlapping unstacked areas |
| Values compared across categories, grouped or stacked | `BarChart` (`normalization={PERCENT}` for shares) | a `PieChart` with many parts |
| Parts of one whole, two to six of them | `PieChart` | `PieChart` for values that are not parts of a whole |
| A value for every pair of two categories (hour × weekday) | `Heatmap` | encoding categories as ramp steps |
| A word-sized trend beside a number | `Sparkline` | a `Sparkline` alone to report values |
| A KPI: a label and a headline number | `StatChart` | a `Widget` with a big `Text` |
| Linked hover across charts that share a category axis | `ChartGroupProvider` | syncing a `PieChart` |

Charts fill their parent (size them with a `Box`, a `Grid` track or a flex item) or take a 2:1 fallback. Every chart has a tooltip, keyboard navigation (Tab, then the arrow keys), `onSelect` / `selection`, `isLoading` and an empty state. `isFilterable` (Line, Area, Bar, Pie, Heatmap) makes clicks filter: a legend entry, a mark or a category pins (click again to unpin, ⌘ / Ctrl / Shift-click to add), hovering a legend entry still previews, and filter chips after the legend name the pin, one per part ("06:00" and "Failed" for a segment): a chip's × drops that part, and several pins collapse into one "3 filters" chip (Escape on the plot clears everything). Uncontrolled by default (`defaultSelection`), or controlled with `selection` + `onSelectionChange(selection)` to filter something else on the page. Series colors come from `ChartPalette` slots (`@galaxy-io/dls/charts/types`), assigned in order when unset. Legend and tooltip swatches mirror the marks (a dot for lines, a square for filled marks); `swatch` (`ChartSwatch`) overrides it, so a line chart beside a bar chart of the same series can share square keys. Requires the `d3-scale` and `d3-shape` peers.

### LineChart

`@galaxy-io/dls/charts/LineChart` · `series` (label and color per metric), `lines` (`metric`, `points` of `{ x, y }`; `null` breaks the line; `isDashed` for a forecast), `categories`, `curve`, `valueFormatter`, `labelFormatter`, `valueDomain`, `axisLabels`, `hasLegend`, `swatch`, `hasTooltip`.

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import LineChart from "@galaxy-io/dls/charts/LineChart";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import { formatNumber } from "@galaxy-io/dls/utils/format";

<Box height={240}>
  <LineChart
    ariaLabel="Signups and activations, last 4 months"
    series={{ signups: { label: "Signups" }, activations: { label: "Activations", color: ChartPalette.TEAL } }}
    lines={[
      { metric: "signups", points: [{ x: "Jun", y: 120 }, { x: "Jul", y: 180 }, { x: "Aug", y: 240 }, { x: "Sep", y: 260 }] },
      { metric: "activations", points: [{ x: "Jun", y: 60 }, { x: "Jul", y: 90 }, { x: "Aug", y: null }, { x: "Sep", y: 150 }] },
    ]}
    valueFormatter={(value) => formatNumber(value, { compact: true })}
  />
</Box>;
```

### AreaChart

`@galaxy-io/dls/charts/AreaChart` · LineChart's engine with flat translucent fills: `series`, `areas` (`metric`, `points`), `isStacked` (parts of a total).

```tsx
import AreaChart from "@galaxy-io/dls/charts/AreaChart";
import Box from "@galaxy-io/dls/layout/Box";

<Box height={240}>
  <AreaChart
    isStacked
    series={{ organic: { label: "Organic" }, paid: { label: "Paid" } }}
    areas={[
      { metric: "organic", points: [{ x: "Mon", y: 40 }, { x: "Tue", y: 52 }, { x: "Wed", y: 48 }] },
      { metric: "paid", points: [{ x: "Mon", y: 20 }, { x: "Tue", y: 18 }, { x: "Wed", y: 25 }] },
    ]}
  />
</Box>;
```

### BarChart

`@galaxy-io/dls/charts/BarChart` · `series` (metrics), `groups` (categories) → `bars` (one per metric) → `components` (stacked segments: `key`, `label`, `value`). Grouping and stacking come from the data. `normalization={BarChartNormalization.PERCENT}` makes 100% stacks; `minSegmentLength` keeps tiny segments visible.

```tsx
import BarChart from "@galaxy-io/dls/charts/BarChart";
import Box from "@galaxy-io/dls/layout/Box";

<Box height={240}>
  <BarChart
    series={{ runs: { label: "Runs" } }}
    groups={[
      { label: "Mon", bars: [{ metric: "runs", components: [{ key: "ok", label: "Succeeded", value: 42 }, { key: "failed", label: "Failed", value: 3 }] }] },
      { label: "Tue", bars: [{ metric: "runs", components: [{ key: "ok", label: "Succeeded", value: 39 }, { key: "failed", label: "Failed", value: 6 }] }] },
    ]}
  />
</Box>;
```

### PieChart

`@galaxy-io/dls/charts/PieChart` · Parts of one whole: `series` (slices), `slices` (`key`, `value`), `isSolid` (default is a donut), `minSliceShare` (small slices fold into "Other"), `children` (content in the donut hole). Never joins chart sync.

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import PieChart from "@galaxy-io/dls/charts/PieChart";
import Text from "@galaxy-io/dls/text/Text";

<Box height={200}>
  <PieChart
    series={{ free: { label: "Free" }, team: { label: "Team" }, enterprise: { label: "Enterprise" } }}
    slices={[
      { key: "free", value: 620 },
      { key: "team", value: 240 },
      { key: "enterprise", value: 40 },
    ]}
  >
    <Text>900 accounts</Text>
  </PieChart>
</Box>;
```

### Heatmap

`@galaxy-io/dls/charts/Heatmap` · A grid of cells whose fill strengthens with the value: `cells` (`column`, `row`, `value`; `null` is empty), `columns`, `rows`, `color` (one `ChartPalette` hue whose meaning fits the value), `valueDomain` (share it to compare two heatmaps).

```tsx
import Box from "@galaxy-io/dls/layout/Box";
import Heatmap from "@galaxy-io/dls/charts/Heatmap";
import { ChartPalette } from "@galaxy-io/dls/charts/types";

<Box height={160}>
  <Heatmap
    color={ChartPalette.ERROR}
    rows={["Mon", "Tue"]}
    columns={["00", "06", "12", "18"]}
    cells={[
      { row: "Mon", column: "00", value: 0 },
      { row: "Mon", column: "12", value: 4 },
      { row: "Tue", column: "06", value: 1 },
      { row: "Tue", column: "18", value: null },
    ]}
  />
</Box>;
```

### Sparkline

`@galaxy-io/dls/charts/Sparkline` · A word-sized line or bar trend (`mark`) with no axes or interaction: `data`, `variant` (default `SECONDARY`) or `color`, `size` (the row's rung; four times as wide), `fillWidth`, `valueDomain`. Decorative without `ariaLabel`; pair it with the number it summarizes.

```tsx
import Sparkline, { SparklineVariant } from "@galaxy-io/dls/charts/Sparkline";

<Sparkline data={[3, 5, 4, 8, 7, 9, 12]} variant={SparklineVariant.SUCCESS} ariaLabel="Signups, last 7 days, rising" />;
```

### StatChart

`@galaxy-io/dls/charts/StatChart` · The KPI tile: `label`, `value`, `description` (the comparison), `trailing` (a `Sparkline` or a status `Chip`), `variant` (surface), `hasBorder` (tiles on a page; off inside a bordered card), `isLoading`. Don't color the headline with a status.

```tsx
import Sparkline from "@galaxy-io/dls/charts/Sparkline";
import StatChart from "@galaxy-io/dls/charts/StatChart";

<StatChart label="Rows synced" value="1.2M" description="+4% vs last week" trailing={<Sparkline data={[4, 6, 5, 8, 9]} />} hasBorder />;
```

### ChartGroupProvider

`@galaxy-io/dls/charts/ChartGroupProvider` · Links the `LineChart`, `AreaChart` and `BarChart` inside it: hovering a category in one shows it in every sibling (matched by raw category label). `shouldShareTooltip` opens every tooltip. Renders no DOM.

```tsx
import BarChart from "@galaxy-io/dls/charts/BarChart";
import ChartGroupProvider from "@galaxy-io/dls/charts/ChartGroupProvider";
import LineChart from "@galaxy-io/dls/charts/LineChart";
import Grid from "@galaxy-io/dls/layout/Grid";

<ChartGroupProvider>
  <Grid columns={2} gap={16} height={200}>
    <LineChart series={{ p95: { label: "p95 ms" } }} lines={[{ metric: "p95", points: [{ x: "10:00", y: 120 }, { x: "11:00", y: 140 }] }]} />
    <BarChart
      series={{ requests: { label: "Requests" } }}
      groups={[
        { label: "10:00", bars: [{ metric: "requests", components: [{ key: "requests", label: "Requests", value: 900 }] }] },
        { label: "11:00", bars: [{ metric: "requests", components: [{ key: "requests", label: "Requests", value: 1100 }] }] },
      ]}
    />
  </Grid>
</ChartGroupProvider>;
```

`@galaxy-io/dls/charts/types` holds the shared chart types and enums: `ChartPalette`, `ChartCurve`, `ChartSeriesStyle`, `ChartSelection`, `ChartTooltipRenderer`, … The other `charts/*` modules are building blocks; see [Low-level modules](#low-level-modules).

## Editor

### CodeEditor

`@galaxy-io/dls/editor/CodeEditor` · CodeMirror 6 in the DLS field frame: `value` / `defaultValue` / `onChange`, `language` (`CodeEditorLanguage`), `onExecute` (⌘/Ctrl+Enter), `hasLineNumbers`, `isReadOnly`, `isError`, `isGhost`, `variant`, `placeholder`, `extensions` (memoize the array). It fills its parent, so give the parent a height; label it with a `Field` or `ariaLabel`. Read-only code to display is `CodeBlock`. Requires the CodeMirror peers.

`@galaxy-io/dls/editor/CodeEditorReactWidget` is the abstract base class for CodeMirror widgets that render React content inline (a `Chip` for a template variable), for decorations in your own `extensions`. Subclass it and implement `render()` and `eq()`. The content renders in its own React root: tokens reach it, provider context does not.

```tsx
import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
import Field from "@galaxy-io/dls/inputs/Field";
import Box from "@galaxy-io/dls/layout/Box";

<Field label="Query" description="⌘ Enter runs it." fillWidth>
  <Box height={200}>
    <CodeEditor
      language={CodeEditorLanguage.SQL}
      defaultValue="select * from orders limit 10;"
      hasLineNumbers
      onExecute={(sql) => console.log(sql)}
    />
  </Box>
</Field>;
```

## Brand

All marks paint `text.primary` (monochrome), follow the theme in CSS, and take `isInverse` for a mark on a `text.primary` fill. `size` is the height in px. The artwork is one SVG per mark in `src/brand/assets/`, compiled by `pnpm assets` into a private component the public mark renders.

| Module | What | When |
|---|---|---|
| `@galaxy-io/dls/brand/GalaxyLogomark` | The two-circle, one-ribbon mark | Sidebar headers, favicons in UI, compact brand spots. Decorative unless `ariaLabel`. |
| `@galaxy-io/dls/brand/GalaxyWordmark` | "galaxy" in the brand lettering | Beside the logomark when there is room. |
| `@galaxy-io/dls/brand/GalaxyFilamentWordmark` | "filament" in the same lettering | Filament's product wordmark. |
| `@galaxy-io/dls/brand/GalaxyLogomarkAnimation` | The mark drawing itself in a loop | Splash and long-loading moments; `isPaused`; needs `lottie-react`. |
| `@galaxy-io/dls/brand/GalaxyLogomark3D` | The mark as a lit 3D object | Hero moments only; `isAutoRotate`, `isInteractive`; needs `three` (loaded after mount from `src/brand/assets/galaxy-logomark.obj`). |
| `@galaxy-io/dls/three/galaxyLogomark3DScene` | The three.js scene behind it | Mounting the 3D mark into your own canvas. |
| `@galaxy-io/dls/brand/brandMark` | Shared CSS and ARIA helpers for the marks | Building another mark. |

```tsx
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import GalaxyWordmark from "@galaxy-io/dls/brand/GalaxyWordmark";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";

<Flex gap={8} alignItems={AlignItems.CENTER}>
  <GalaxyLogomark size={20} />
  <GalaxyWordmark size={16} />
</Flex>;
```

## Transforms

Enter, exit and state motion, all at `duration.base` (150ms) and instant under reduced motion. Keep them mounted and drive them with their boolean; never render them conditionally (they could not animate out).

| Module | Motion | Use |
|---|---|---|
| `@galaxy-io/dls/transform/Fade` | Opacity in and out | Content that appears and disappears in place. `isOpen`, `shouldKeepMounted`, `shouldAnimateOnMount`. |
| `@galaxy-io/dls/transform/ScaleFade` | Opacity plus 95% → 100% scale | Floating surfaces that pop (your own popover or card). Not inline content. |
| `@galaxy-io/dls/transform/Rotate` | Turn to `deg` (default 180) | A caret or sort arrow showing a state change. `isRotated`. |
| `@galaxy-io/dls/transform/Flasher` | Pulse between full and 40% opacity | A value that exists and is updating. `isFlashing`. Not for content that has not loaded (`Skeleton`). |

```tsx
import { CaretDownIcon } from "@phosphor-icons/react";
import Icon from "@galaxy-io/dls/icons/Icon";
import Alert from "@galaxy-io/dls/feedback/Alert";
import Fade from "@galaxy-io/dls/transform/Fade";
import Flasher from "@galaxy-io/dls/transform/Flasher";
import Rotate from "@galaxy-io/dls/transform/Rotate";
import Text from "@galaxy-io/dls/text/Text";

declare const isOpen: boolean;
declare const isSyncing: boolean;

<>
  <Rotate isRotated={isOpen}>
    <Icon component={CaretDownIcon} size={12} />
  </Rotate>
  <Fade isOpen={isOpen}>
    <Alert>Changes apply to the next run.</Alert>
  </Fade>
  <Text>
    <Flasher isFlashing={isSyncing}>48,210 rows</Flasher>
  </Text>
</>;
```

## Theme and setup

| Module | What |
|---|---|
| `@galaxy-io/dls/styles.css` | Component CSS and the package reset. Import once. |
| `@galaxy-io/dls/tokens.css` | The `--gx-*` variables. Import once. |
| `@galaxy-io/dls/fonts.css` | The `@font-face` rules. Import once. |
| `@galaxy-io/dls/theme/GalaxyProvider` | The root provider: theme, overlay stack, toasts. See [getting-started.md](https://github.com/galaxy-io/dls/blob/main/docs/getting-started.md#galaxyprovider). |
| `@galaxy-io/dls/theme/ThemeSwitcher` | The System / Dark / Light control (`size`, `variant`, `isIconOnly`, `label`). |
| `@galaxy-io/dls/theme/useGalaxyTheme` | `selectedTheme`, `activeTheme`, `systemTheme`, `setTheme`, `reducedMotion`, `theme` (hex). |
| `@galaxy-io/dls/theme/enums` | `GalaxyTheme`, `ReducedMotion`, `FontFamily`, `Placement`, `Orientation`, `Side`, `Radius`, the `Space` type, the canonical member lists. |
| `@galaxy-io/dls/theme/tokens/t` | The typed token accessor. |
| `@galaxy-io/dls/theme/tokens/types` | `Theme`, `Scales`, `PaletteColor`, `StatusColor`, `Hex`, … |
| `@galaxy-io/dls/theme/tokens/dark`, `@galaxy-io/dls/theme/tokens/light` | `DARK_THEME`, `LIGHT_THEME`: resolved hex, for JS logic. |
| `@galaxy-io/dls/theme/tokens/palette` | `PALETTE`, `PALETTE_COLORS`, `PALETTE_STEPS`: the mode-stable palette, for charts. |
| `@galaxy-io/dls/theme/tokens/scales` | `SPACE`, `RADIUS`, `DURATION`, `Z`, `CONTROL_SIZE`, `TYPE_SCALE`. |
| `@galaxy-io/dls/theme/tokens/generate` | `tokenName`, `themeToCss`, `scalesToCss`: how `tokens.css` is built. |
| `@galaxy-io/dls/theme/useInverseScope` | Internal: the attributes of an inverse scope (the Tooltip bubble). Apps use a scoped `GalaxyProvider`. |
| `@galaxy-io/dls/styles/mixins` | `INTERACTIVE_RESET`, `HAIRLINE_BORDER`, `HAIRLINE_WIDTH` (1px), `FOCUS_RING`, `FIELD_FOCUS`, `TRUNCATE`, `VISUALLY_HIDDEN`, `REDUCED_MOTION`. |
| `@galaxy-io/dls/vite` | `galaxyDls({ prefix })`: the Linaria (wyw-in-js) setup for apps. |
| `@galaxy-io/dls/biome` | The Biome preset: `"extends": ["@galaxy-io/dls/biome"]`. |

```tsx
import ThemeSwitcher from "@galaxy-io/dls/theme/ThemeSwitcher";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

export function ChartBackground() {
  const { theme } = useGalaxyTheme();
  return <canvas data-background={theme.color.background.base} />;
}

<ThemeSwitcher isIconOnly />;
```

## Hooks and utils

All SSR-safe (nothing touches `window`, `document` or `localStorage` outside effects). Apps use these instead of re-implementing them. Every hook in `src/hooks/`, with its typed signature, SSR notes and a live demo, is on the Storybook page Home → Hooks.

| Module | What |
|---|---|
| `@galaxy-io/dls/hooks/useDisclosure` | Open/closed state for a modal, drawer or disclosure: `{ isOpen, open, close, toggle, setIsOpen }`; `setIsOpen` fits `onOpenChange`. |
| `@galaxy-io/dls/hooks/useClipboard` | Copies text to the clipboard: `{ copy, hasCopied }`; `hasCopied` holds for 1.5s after a success. |
| `@galaxy-io/dls/hooks/useHotkey` | Binds a keyboard shortcut: `useHotkey("mod+k", handler, { scope, shouldPreventDefault, shouldIgnoreInputs, isEnabled })`; the innermost scope wins. Also `parseHotkey`, `matchesHotkey`. |
| `@galaxy-io/dls/hooks/useIsMac` | Whether the user is on an Apple platform, for ⌘ vs Ctrl labels and bindings. |
| `@galaxy-io/dls/hooks/useMediaQuery` | Whether a CSS media query matches, updating live. |
| `@galaxy-io/dls/hooks/usePrefersReducedMotion` | Whether to reduce motion: the provider's `reducedMotion`, else `prefers-reduced-motion`. |
| `@galaxy-io/dls/hooks/useLocalStorage` | State persisted in `localStorage` under a key: `[value, setValue, remove]`, synced across instances and tabs. |
| `@galaxy-io/dls/hooks/useControlledState` | State that is controlled when `value` is set and internal otherwise: `[value, setValue]`, the engine of every `value` / `defaultValue` / `onChange` triple. |
| `@galaxy-io/dls/hooks/useControlledOpenState` | The same for overlays and disclosures: `{ isOpen, setOpen }` from `isOpen` / `defaultIsOpen` / `onOpenChange`. |
| `@galaxy-io/dls/hooks/useDebouncedValue` | A value once it has stopped changing for a delay, for search-as-you-type. |
| `@galaxy-io/dls/hooks/useDismiss` | Closes an app-built floating element on Escape or an outside mousedown; returns its ref. DLS overlays do not need it. |
| `@galaxy-io/dls/hooks/useIsomorphicLayoutEffect` | `useLayoutEffect` in the browser, `useEffect` on the server (no SSR warning), for code that measures or focuses before paint. |
| `@galaxy-io/dls/hooks/useElementWidth` | An element's border-box width, live (`ResizeObserver`), for charts and self-sizing layouts. |
| `@galaxy-io/dls/hooks/useElementClientSize` | An element's client width and height (no borders or scrollbars), live, for things that must line up with the scrollport. |
| `@galaxy-io/dls/hooks/useResize` | Drag-resizing from a handle on an edge or corner (`ResizeAnchor`), for custom resizable surfaces. |
| `@galaxy-io/dls/hooks/useField` | The `Field` wiring (ids, `aria-describedby`, required, error) for a custom control inside a Field (see [useField](#usefield)). |
| `@galaxy-io/dls/utils/format` | `formatNumber` (`1,234`, `{ compact: true }` → `1.2k`), `formatBytes`, `formatDuration`, `formatRelativeTime`, `formatPercent`; `en-US` by default so server and client agree. |
| `@galaxy-io/dls/utils/colors` | Hex / RGB / HSL conversion, `mix`, `contrastRatio`, `meetsAA`, for data-driven colors. |
| `@galaxy-io/dls/utils/css` | `getCssSizeValue`, `spaceToCss`, `gapToCss`: what the layout components use to turn props into CSS. |

```tsx
import { useRef } from "react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useClipboard } from "@galaxy-io/dls/hooks/useClipboard";
import { useDebouncedValue } from "@galaxy-io/dls/hooks/useDebouncedValue";
import { useHotkey } from "@galaxy-io/dls/hooks/useHotkey";
import { useLocalStorage } from "@galaxy-io/dls/hooks/useLocalStorage";
import { formatBytes, formatRelativeTime } from "@galaxy-io/dls/utils/format";

export function Example({ query, url }: { query: string; url: string }) {
  const searchRef = useRef<HTMLInputElement>(null);
  const [isCollapsed, setIsCollapsed] = useLocalStorage("sidebar:collapsed", false);
  const debouncedQuery = useDebouncedValue(query, 200);
  const { copy, hasCopied } = useClipboard();
  useHotkey("mod+b", () => setIsCollapsed(!isCollapsed), { shouldPreventDefault: true });
  useHotkey("/", () => searchRef.current?.focus(), { shouldIgnoreInputs: true });
  return (
    <Button
      label={hasCopied ? "Copied" : `Copy link (${formatBytes(2048)}, ${formatRelativeTime(Date.now() - 6e4)}, ${debouncedQuery})`}
      variant={ButtonVariant.SECONDARY}
      onClick={() => void copy(url)}
    />
  );
}
```

## Accessibility primitives

| Module | What |
|---|---|
| `@galaxy-io/dls/a11y/FocusScope` | Keeps focus inside a region: moves in on mount, wraps Tab, restores focus on unmount, `onEscape`. Overlays already use it; wrap your own modal-like surfaces. |
| `@galaxy-io/dls/a11y/VisuallyHidden` | Content for screen readers only (a hidden label, a live-region message). |

```tsx
import FocusScope from "@galaxy-io/dls/a11y/FocusScope";
import VisuallyHidden from "@galaxy-io/dls/a11y/VisuallyHidden";
import Button from "@galaxy-io/dls/buttons/Button";

<FocusScope onEscape={() => {}}>
  <Button label="Done" />
  <VisuallyHidden>Press Escape to close.</VisuallyHidden>
</FocusScope>;
```

## Low-level modules

Exported so apps can build charts and tables the components do not cover. They follow the same conventions but are not part of the everyday surface; prefer the components above.

| Module | What |
|---|---|
| `@galaxy-io/dls/charts/ChartFrame` | The chart root: sizing, the plot box, loading and empty states. |
| `@galaxy-io/dls/charts/ChartAxis` | `ChartValueAxis`, `ChartCategoryAxis`. |
| `@galaxy-io/dls/charts/ChartLegend` | The legend, with overflow ("+N more"). |
| `@galaxy-io/dls/charts/ChartTooltip` | The chart tooltip body and positioning. |
| `@galaxy-io/dls/charts/ChartCategoryTargets` | Hit targets per category (hover, click, keyboard). |
| `@galaxy-io/dls/charts/ChartMarkTargets` | Hit targets per mark. |
| `@galaxy-io/dls/charts/ChartPrimitives` | CSS for lines, areas, bars, arcs, points, grid lines and cursors. |
| `@galaxy-io/dls/charts/constants` | The shared chart spacing and geometry constants. |
| `@galaxy-io/dls/charts/chartPalette` | Slot order and the slot → color mapping. |
| `@galaxy-io/dls/charts/chartFormat` | Default value and label formatters. |
| `@galaxy-io/dls/charts/chartScales` | Value domains, scales, ticks and insets. |
| `@galaxy-io/dls/charts/barChartGeometry`, `@galaxy-io/dls/charts/barChartLegend` | Bar layout on whole pixels; bar legend items. |
| `@galaxy-io/dls/charts/lineChartGeometry` | Line and area paths, stacking. |
| `@galaxy-io/dls/charts/pieChartGeometry` | Slices, folding, arcs. |
| `@galaxy-io/dls/charts/heatmapGeometry` | Cells, ramp steps, labels. |
| `@galaxy-io/dls/charts/useChartDimensions`, `@galaxy-io/dls/charts/useCartesianChartLayout` | Measuring the chart and laying out cartesian plots. |
| `@galaxy-io/dls/charts/useChartInteraction`, `@galaxy-io/dls/charts/useChartTooltipPosition` | Hover, keyboard and selection state; tooltip placement. |
| `@galaxy-io/dls/charts/chartSelection` | The click-to-filter model: toggling descriptors, additive clicks. |
| `@galaxy-io/dls/charts/useChartFormatters`, `@galaxy-io/dls/charts/useSeriesColorResolver` | Resolved formatters; series colors. |
| `@galaxy-io/dls/tree/treeRow` | The row geometry Tree and JsonViewer share. |
| `@galaxy-io/dls/table/TableCore` and the `table/use*` hooks | See [Table building blocks](#table-building-blocks). |
