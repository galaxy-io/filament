# Screens

The shells Filament already has, and how a new screen fits into one. Reuse these before building a frame. Every screen is one of a handful of shapes.

| You are building | Shell | Reference screen |
|---|---|---|
| A top-level list (one row or card per entity, search, a primary "New X") | `MainLayout` → `MainLayoutListPage` | `pages/pipelines/PipelinesPage.tsx`, `pages/connectors/ConnectionsPage.tsx` |
| A dashboard | `MainLayout` with a `BaseToolbar` and a scrolling column of `Widget`s | `pages/observability/ObservabilityPage.tsx`, see [dashboards.md](./dashboards.md) |
| A detail area with its own sub-navigation | `PipelineLayout` (icon sidebar, navbar, content island) | `pages/pipelines/PipelinePage.tsx` and its canvas / history / settings children |
| A settings surface for one entity | `BaseHeader` + `Divider` + a scrolling column of `Widget` sections, each with its own Save | `pages/pipelines/PipelineSettingsPage.tsx` |
| Details of an item while the list stays visible | DLS `Drawer` opened by a search param | `pages/connectors/components/drawer/ConnectionDrawer.tsx` |
| A task the user must finish or abandon | DLS `Modal` opened by a `Flow` param, a `ConnectionFormWrapper`-style shell | `pages/connectors/components/edit/EditConnectionModal.tsx`, `CreatePipelineModal.tsx` |
| A yes/no before an action | `useConfirm` + DLS `ConfirmDialog` (rows) or `components/Dialog` (page-level danger zone) | `ConnectionDrawer.tsx`, `pages/pipelines/settings/PipelineSettingsPageDanger.tsx` |
| App-wide settings | the `SettingsPage` modal with a `SidebarNav` of panels | `pages/settings/SettingsPage.tsx` |

## The frame

`MainLayout` is a 48px navbar over a body with a 12px gutter, and inside the gutter the **island**, `background.primary` with a hairline border and `t.radius.lg`, `overflow: hidden`. Pages render inside the island and own their scrolling.

The navbar (`MainLayoutNavbar`) is three rails. The Filament wordmark and a Docs link on the left, DLS `Tabs` with `as={RouterLink}` for Observability / Pipelines / Sources / Sinks in the middle, and the account `Popover` (plus GitHub and theme controls when auth is off) on the right. A new top-level destination is one entry in `MAIN_NAVBAR_ITEMS` plus a route under `_main`.

`PipelineLayout` is the detail frame. A 48px left column with a back button and an icon-only sidebar (Canvas / History / Settings, driven by `PipelineSidebarItem` and its `_TO_ICON_MAP` / `_TO_LABEL_MAP`), a navbar with the pipeline's name, version select and run controls, and the island on the right. When a past version is previewed the island's border turns `border.error` and a red "Version N" chip floats in its corner.

## List page

```tsx
const PipelinesPage = () => {
  const navigate = useNavigate();
  const search = useSearch({ from: "/_app/_main/pipelines" });

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useSuspenseListPipelinesInfiniteQuery({ input: createListPipelinesInput(search) });
  const pipelines = useMemo(() => data.pages.flatMap((page) => page.pipelines), [data.pages]);

  const handleNewPipeline = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId: undefined, flow: Flow.CREATE_PIPELINE }),
    });
  };

  const renderContent = () => {
    if (!pipelines.length && !search.q) {
      return <PipelinesPageEmptyGraphic actions={…} />;
    }
    return <PipelinesTable pipelines={pipelines} … />;
  };

  return (
    <MainLayoutListPage
      actions={[<Button key="new-pipeline" label="New pipeline" icon={PlusIcon} variant={ButtonVariant.PRIMARY} onClick={handleNewPipeline} />]}
      noPadding
    >
      {renderContent()}
    </MainLayoutListPage>
  );
};
```

- **`MainLayoutListPage` owns the search box.** It reads `q`, debounces 300ms and writes it back with `replace: true`. The page passes `actions` (an array of keyed nodes, the one `PRIMARY` button last) and children. `noPadding` for a full-bleed table, default 12px padding for a card grid.
- **Two empty states.** Nothing at all and no search → the feature's `*PageEmptyGraphic` (ghost tiles built from `components/EmptyGraphic`) with a `LARGE` primary button and a `DocsLink`. Nothing matching a search → `EmptyLayout` with a `MagnifyingGlassIcon` and "No pipelines match your search", passed to the table's `emptyState` or rendered in place of the grid.
- **Table or grid.** Entities with many scannable columns are an `InfiniteTable` (pipelines, runs, members). Entities that are mostly a tile and a name are a `Grid` of cards (`columns={`repeat(auto-fill, minmax(${MIN}px, 1fr))`}`) followed by `InfiniteScrollSentinel`.
- **Row click opens the entity.** `onRowClick` navigates to the detail route, or writes the entity's id into the drawer param. Row actions live in `rowActions` as `MenuItem`s inside a presentational `*TableRowActions` component that takes `onX(row)` callbacks. The table owns the mutations.
- **Sorting is URL state.** `sort` / `onSortChange` map between column ids and the proto `SortBy` in the table's `utils.ts`, and the default sort is written as `undefined`.

### Table conventions

- Columns are a module-level `TableColumn<Row>[]` const named `X_TABLE_COLUMNS` when static, or a `createXColumns(deps)` factory in `useMemo` when they depend on data. Widths are constants in the folder's `constants.ts` (`PIPELINES_TABLE_COLUMN_WIDTH_STATUS = 140`).
- Cells are `Text size={TextSize.BODY_SM} lineClamp={1}`. Numbers, ids and durations add `family={FontFamily.MONO}` and the column is `align: "right"`. A missing value is an em dash `—`. Muted states are `TextVariant.TERTIARY` ("Never run").
- A cell with its own logic is its own component in `columns/` (`PipelinesTableColumnRecentRuns`) or `*Cell.tsx`.
- `getRowId`, `ariaLabel`, `isRowHeader` on the naming column. `canSort` only where the server sorts.
- The table sits in a wrapper with `flex: 1; min-height: 0` and a `Box height="100%" fillWidth`, so it scrolls inside the island. `isLoading={isFetchingNextPage}` and `onEndReached` page in.
- `canCustomizeColumns` + `columnLayout` persisted through `useLocalStorage` (key `filament:<table>:column-layout`) and debounced.
- Expanded rows are URL state (`?runId=`) through `expandedIds` / `onExpandedIdsChange`, and the expanded component fetches its own entity with a plain query.

## Detail and settings pages

A page inside `PipelineLayout` is a column. `PageWrapper` (full size, `overflow: hidden`, `background.base`), a `Box padding={16}` holding `BaseHeader size={BaseHeaderSize.LARGE} title="Settings"`, a `Divider`, then a `ScrollWrapper` (`flex: 1; min-height: 0; overflow-y: auto`).

```tsx
<ScrollWrapper>
  <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} padding={16} minWidth={400} maxWidth={640}>
    <PipelineSettingsPageGeneral />
    <PipelineSettingsPageSchedule />
    <PipelineSettingsPageNotifications pipeline={data.pipeline} />
    <PipelineSettingsPageAdvanced />
    <PipelineSettingsPageDanger />
  </Flex>
</ScrollWrapper>
```

- **The page composes flat section components.** No intermediate `*Form`. Each section fetches the entity itself and returns `null` when it does not apply (the schedule section for a continuous pipeline).
- **A section is a collapsible `Widget`** (`isCollapsible header="General" defaultIsOpen`). Related fields share one widget, a standalone toggle gets its own. `gap={12}` between widgets.
- **Every section has local state applied by one Save.** Toggles included. No immediate-action RPCs on a settings page. The Save / Cancel row is right-aligned, Cancel `SECONDARY` disabled unless dirty, Save `PRIMARY` with `isDisabled={!canSave}` and `isLoading={isSaving}`. Dirty and valid are derived at render. See [forms.md](./forms.md).
- **No add-step friction.** Fields are always visible. No "Add X" empty state for cheap config and no remove button for config that is trivially re-created.
- **Danger zone** is a plain `Widget` with a space-between row, a medium-weight title over a `BODY_SM` `SECONDARY` description, and a `ButtonVariant.ERROR` button with `TrashIcon`. It confirms through `useConfirm` and `components/Dialog` with `confirmationPhrase` set to the entity's name.
- `BaseHeader` (`layouts/components/BaseHeader.tsx`) is the house title row for pages, panels and dropdowns. `title`, optional `icon`, `description`, `actions` and `onClose`. Sizes `SMALL` (dropdown panels), `MEDIUM`, `LARGE` (page headers). `BaseToolbar` is the matching row of `leadingActions` and `trailingActions`.

## Drawer

```tsx
<Drawer
  size={DrawerSize.MEDIUM}
  isOpen={isOpen}
  onOpenChange={(open) => { if (!open) onClose(); }}
  header={connection ? <ConnectionDrawerHeader connection={connection} /> : "Connection"}
  actions={connection && (
    <Menu trigger={<Button icon={DotsThreeIcon} ariaLabel="Connection actions" variant={ButtonVariant.TERTIARY} size={ButtonSize.SMALL} />}>
      <MenuItem label="Edit connection" icon={PencilIcon} onSelect={handleEdit} />
      <MenuSeparator />
      <MenuItem label="Delete connection" icon={TrashIcon} variant={MenuItemVariant.ERROR} onSelect={() => handleOpen(connection)} />
    </Menu>
  )}
>
  {renderContent()}
  <ConfirmDialog isOpen={isOpen && confirmIsOpen} … />
</Drawer>
```

- Opened by `?connectionId=`, owned by `AppLayout`, id held with `useRetainedWhileClosed`. The drawer takes `connectionId`, `isOpen`, `onClose` as props and reads no search params. See [routing.md](./routing.md).
- `renderContent()` checks `isError` (an `ErrorLayout` with a Close button), then `!entity` (`PendingLayout`), then the content.
- Actions on the thing the drawer shows go in `actions` as a ⋯ `Menu`, Delete last and red. A read-only drawer has no footer.
- Body building blocks, reused by the canvas panel too. `ConnectionDrawerList` (a bordered list with hairline separators between `ConnectionDrawerKeyValueRow`s, label medium on the left and a node on the right), `ConnectionDrawerSection` (a collapsible `Widget` with a count chip in `actions`, `isFlush` when it has items, `EmptyLayout size={LayoutSize.SMALL}` when it does not), `ConnectionDrawerJsonSection` (a section around `CodeBlock language=JSON canCopy`).
- A related-entities list inside a drawer is a stack of compact `PipelineCard` rows (40px, hairline bottom border, hover background), not a table.

## Modal

- Opened by `?flow=`, rendered by `AppLayout`, mounted conditionally so its reducer resets per open. The modal takes `onClose` and maps `onOpenChange(false)` to it. `isOpen` is always true while mounted.
- `ModalSize.MEDIUM` for a form or a dialog, `X_LARGE` for a chooser or a multi-step wizard, which lays out a sidebar and body inside a `FrameWrapper` (hairline border, `t.radius.lg`).
- The same shell shows every state. Error → `ErrorLayout` with a Close button inside the modal body. Loading → `PendingLayout` inside the body. Loaded → the provider and the content component.
- `header` is a string, or a header component with a `ConnectorTile` and a `BaseHeader size={BaseHeaderSize.LARGE}` for connector forms. Title copy is `${isEdit ? "Edit" : "New"} ${displayName} connection`.
- The footer is end-aligned, Cancel or Back `SECONDARY` first and the one `PRIMARY` action last. A pending action reads as a gerund with an ellipsis, "Saving...", `isLoading isDisabled onClick={NOOP}`.
- A wizard's sidebar is a DLS `Stepper` (vertical, `SMALL`) in a `Widget variant={WidgetVariant.SECONDARY}`, with completed steps summarised in `description` and the current step's description plus a `DocsLink` beneath. The footer shows a red "Invalid" `Chip` whose `tooltip` is a `BulletedList` of blocking hints while Next is disabled.

## Confirmation

```tsx
const { handleOpen, isOpen, target, handleClose, handleConfirm } = useConfirm<Connection>({
  entityLabel: "Connection",
  entityName: (c) => c.name,
  onConfirm: (c, { onSuccess, onError }) => deleteConnection({ id: c.id }, { onSuccess, onError }),
  onConfirmed: (c) => navigate({ to: c.kind === ConnectorKind.SINK ? "/sinks" : "/sources" }),
});

<ConfirmDialog
  isOpen={isOpen}
  onOpenChange={(next) => { if (!next) handleClose(); }}
  onConfirm={handleConfirm}
  header="Delete connection?"
  description="This deletes the connection. It cannot be undone."
  confirmValue={target?.name}
  label="Delete connection"
  isDestructive
/>
```

- `useConfirm` (`src/hooks/useConfirm.ts`) holds the target, runs the mutation, toasts success ("Connection deleted") and failure ("Delete failed"), closes, then calls `onConfirmed`. Override copy through `messages`.
- Rows and drawer items use DLS `ConfirmDialog` with `isDestructive`, a question header, a consequence description, a verb label and `confirmValue` for type-to-confirm.
- A page-level danger zone uses `components/Dialog` with `confirmationPhrase`, `confirmVariant={ButtonVariant.ERROR}`, `isPending` from the mutation, and optionally `variant={DialogVariant.WARNING}` with `bodyTitle` for a caveat. `Dialog` is also the generic info dialog ("Service account created" with credentials and a Done footer).
- `ConfirmDialog` is gated on the host's `isOpen` too (`isOpen && confirmIsOpen`) so it closes with its drawer.

## State layouts

Three components in `src/layouts/` share one `LayoutSize` scale (`SMALL` / `MEDIUM` / `LARGE`) through the `LAYOUT_SIZE_TO_*_MAP` constants. Never re-derive a per-layout size.

| | Props | Use |
|---|---|---|
| `PendingLayout` | `size`, `message` | Whole-view loading. The router's pending component, a drawer or modal body before its entity arrives, a form body before its schema arrives. The animated Galaxy logomark. |
| `EmptyLayout` | `size`, `icon: ReactNode`, `header`, `message`, `actions` | Nothing to show. `icon` is a node because empty states sometimes use an illustration (`EmptyGraphic`). |
| `ErrorLayout` | `size`, `icon: PhosphorIcon`, `header`, `message`, `error`, `actions` | Something failed, including not found. Renders the icon itself at `IconVariant.ERROR` and shows `error.message` in mono in dev. Always red. |

- Whole-view loading is `PendingLayout`. Content-shaped loading is a DLS `Skeleton` sized to the value (`<Box width={160}><Skeleton /></Box>`), for table cells, repeated rows, KPI numbers and inline names. Do not hand-roll shimmer scaffolds for a drawer.
- An error branch gets `ErrorLayout`, never an `EmptyLayout` with a red icon. When a component picks between the two from an `error` prop, split it into a tiny `*State` component that chooses.
- Sections inside panels use `EmptyLayout size={LayoutSize.SMALL}` with a header and message. Table `emptyState`s use `EmptyLayout` with an icon and message, or a tertiary `Text` centred at a fixed height for dense dashboard tables.

## Status marks

- **Every run status is a square.** `PipelineRunStatusSwatch` (`pages/pipelines/history/PipelineRunStatusSwatch.tsx`) draws a DLS `Square` from `PIPELINE_RUN_STATUS_TO_HUE_MAP[status]` through `HueSquare` (`components/HueSquare.tsx`). `PipelineHistoryRunStatus` is that square plus a `BODY_SM` label and an info tooltip with the reason in mono. Charts showing run statuses pass `swatch={ChartSwatch.SQUARE}` so legends match.
- **One map owns every status color.** `PIPELINE_RUN_STATUS_TO_HUE_MAP: Record<RunStatus, Hue | null>` in `pages/pipelines/history/constants.ts`. `Hue` is a DLS status color or palette family, `null` is neutral. `hueToSquareMark` and `hueToChartPalette` in `src/utils/hue.ts` translate it, and `PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP` is derived with `mapRecordValues`. Add a status there and nowhere else.
- **`Beacon` is not for run status.** It remains for non-run live state, the connection form's "Connected". The DLS pattern page's "Beacon for the live state of a row" is overridden here.
- **`Chip` is for small meta badges**, a kind, a version, "Deleted", "Unsaved changes", "Next run in 5m". Status through `variant`, category through `color`. Pass `tooltip` on the Chip. Never wrap a Chip in `Tooltip`, which injects an `onClick` and turns the chip into a button.
- Permission-gated actions are hidden or passed as `undefined`, not disabled.

## Formatting and copy

`src/utils/format.ts` takes the proto's `bigint` millis and returns `—` for zero.

| Helper | Output |
|---|---|
| `formatCount(bigint)` | `48,210` |
| `formatBytes(bigint)` | `1.2 GB` |
| `formatTimeAgo(bigint)` | `just now`, `5m ago`, `3h ago`, `2d ago`, `Mar 4` |
| `formatTimeUntil(bigint)` | `in 5m`, `soon` |
| `formatTimestamp(bigint)` | `Mar 4, 3:04:05 PM` |
| `formatDuration(start, end)` | `850ms`, `1.2s`, `3m 4s` |
| `formatSeconds(number)` | same, from seconds |
| `stripDeletedName(name)` | drops the `__deleted__<iso>` suffix |

Entity formatters live in the feature's `utils.ts` (`formatPipelineName(pipeline, includeDeleted = false)`, `formatPipelineScheduleSummary`). Counts with nouns go through `pluralize("sink", n, true)`.

Copy, as the app writes it.

- Sentence case everywhere. Buttons are a verb or verb plus noun ("New pipeline", "Run now", "Delete connection", "Create invite"). Never "OK", "Submit", "Yes".
- Pending labels are a gerund with three dots, "Saving...", "Testing...". Placeholders end with three dots too, "Select a column...", "Enter connection name...".
- Toasts. Success header is noun plus past tense ("Pipeline saved", "Run started", "Role updated"), description names the entity and ends with a period. Error header is "<Verb> failed" ("Save failed", "Run failed"), description from `getErrorMessage` with a "Failed to …" or "Could not …" fallback.
- Confirm dialogs ask a question with the entity ("Delete connection?") and describe the consequence ("This deletes the connection. It cannot be undone.").
- Empty states. Header "No <things> yet" or "No <things>", message a full sentence that says what to do ("Create one to give CLI, CI, and automation access to your organization."). A filtered empty result says "No pipelines match your search".
- Inline validation is short and imperative with no period ("Name is required", "Use an absolute http or https URL").
- Missing values are an em dash `—`. Arrows in summaries are `→`.
- Ids, emails, numbers, commands and log lines are mono. Error text in tooltips is mono and `isSelectable`.
