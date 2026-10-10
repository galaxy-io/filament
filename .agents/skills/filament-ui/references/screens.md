# Screens

The shells Filament has and the rules a screen inside one follows. Reuse a shell before building a frame. Every screen is one of these shapes.

| You are building | Shell | Reference screen |
|---|---|---|
| A top-level list (one row or card per entity, search, a primary "New X") | `PageLayout` with `header`, `actions` and a `ListSearch` `toolbar` | `pages/pipelines/PipelinesPage.tsx`, `pages/connections/ConnectionsPage.tsx` |
| A dashboard | `PageLayout` with `actions`, a `ScrollArea` of `Widget`s | `pages/observability/ObservabilityPage.tsx`, see [dashboards.md](./dashboards.md) |
| An entity with sub-pages | `PageLayout` with a node `header`, `actions`, `banner` and `tabs`, children in the `Outlet` | `pages/pipelines/PipelinePage.tsx` and its canvas, history and settings children |
| A settings surface for one entity | a `ScrollArea` of collapsible `Widget` sections, each with its own Save | `pages/pipelines/PipelineSettingsPage.tsx` |
| Details of an item while the list stays visible | DLS `Drawer` opened by a search param | `pages/connections/components/drawer/ConnectionDrawer.tsx` |
| A task the user must finish or abandon | DLS `Modal` opened by a `Flow` param | `pages/connections/components/edit/EditConnectionModal.tsx`, `pages/pipelines/components/create/CreatePipelineModal.tsx` |
| A yes/no before an action | `useConfirm` + DLS `ConfirmDialog` | `ConnectionDrawer.tsx`, `pages/pipelines/settings/PipelineSettingsPageDanger.tsx` |
| Anything not found | a red `ErrorLayout` with RouterLink buttons | `pages/NotFoundPage.tsx`, `pages/pipelines/PipelineNotFoundPage.tsx` |

## The frame

`FilamentLayout` renders `MainLayout`, which is DLS `AppFrame`. The frame is flat, with no island, gutter, border or radius. `AppFrame` draws a fixed 240px sidebar column, a vertical divider and the content column.

`MainLayoutSidebar` is a DLS `SidebarNav` with `hasDividers={false}`, `ariaLabel` "Filament", one item per `FilamentNavItem` (`module/nav.ts` maps each to a label, icon, path and keywords), and a footer with the host's `sidebarFooter` above DLS `PoweredBy`. A new top-level destination is one `FilamentNavItem` member with an entry in each map, a `FilamentPath`, a `*RouteOptions` and its route files.

Every page renders DLS `PageLayout`. A string `header` renders the `h1`. The 64px header row holds `header` on the left and `actions` on the right, then a divider, then `banner`, `tabs` and the `toolbar` row, then `main` (`BASE`, `overflow: hidden`). A page that scrolls wraps its body in `ScrollArea`.

## List page

```tsx
return (
  <PageLayout
    header="Pipelines"
    actions={<Button label={PIPELINE_CREATE_TITLE} icon={PlusIcon} variant={ButtonVariant.PRIMARY} onClick={handleNewPipeline} />}
    toolbar={<ListSearch placeholder="Search pipelines" />}
  >
    {renderContent()}
  </PageLayout>
);
```

- **`ListSearch` owns the search box.** It reads `q`, debounces `LIST_SEARCH_DEBOUNCE_MS` and writes it back with `replace: true`. The server filters by `q`, and the route's `remountDeps` re-suspends the list.
- **Two empty states.** Nothing at all and no search shows the feature's first-run graphic with a `LARGE` primary button and a `DocsLink`. Nothing matching a search shows `EmptyLayout icon={MagnifyingGlassIcon}` with "No pipelines match your search".
- **Table or grid.** Entities with many scannable columns are an `InfiniteTable`. Entities that are mostly a tile and a name are a `Grid` of cards (`columns={`repeat(auto-fill, minmax(${MIN}px, 1fr))`}`) in a `ScrollArea`, paged by DLS `useEndReached`.
- **Row click opens the entity.** It navigates to the detail page or writes the entity's id into the drawer param. Row actions are `MenuItem`s in a presentational `*TableRowActions` that takes `onX(row)` callbacks. The table owns the mutations.
- **Sorting is URL state.** `createTableSorting` and `createTableSortSearch` (`utils/sort.ts`) map column ids to the proto `SortBy` through the table's `*_SORT_BY_TO_COLUMN_ID_MAP`. The default sort is written as `undefined`.

### Table rules

- Columns are a module-level `TableColumn<Row>[]` when static, or a `createXColumns(deps)` factory in `useMemo` when they depend on data. Widths are constants in the folder's `constants.ts`.
- Cells are `Text size={TextSize.BODY_SM} lineClamp={1}`. Numbers, ids and durations add `family={FontFamily.MONO}` and the column aligns right. A missing value is DLS `EMPTY_VALUE`.
- A cell with its own logic is its own component in `columns/`.
- Pass `getRowId`, `ariaLabel`, and `isRowHeader` on the naming column. `canSort` only where the server sorts.
- A bounded table sits in `Flex direction={COLUMN} grow={1} basis={0} minHeight={0}` and never scrolls the page. `isLoading={isFetchingNextPage}` and `onEndReached` page in.
- Expanded rows are URL state (`?runId=`), and the expanded component fetches its own entity.
- A nested table with no header row uses `hasHeader={false}`, which keeps the header for screen readers.

## Entity page with tabs

`PipelinePage` reads `usePipelineParams`, suspends on `GetPipeline`, throws `notFound()` when the pipeline is missing, and renders `PageLayout` inside the canvas provider. Its header pieces live in `pages/pipelines/components/header/`.

- The heading starts with an icon-only `TERTIARY` back button, `ArrowLeftIcon`, `ariaLabel` and `tooltip` "All pipelines", `href={createFilamentHref(FilamentPath.PIPELINES)}`, `as={RouterLink}`.
- `tabs` are DLS `Tabs` whose items link to `FilamentPath` children.
- `banner` is one `Alert` at a time. A previewed version shows the preview banner, otherwise a scheduled pipeline shows `Alert color="yellow"` with the next run.
- The children render in the `Outlet` and fill `main`.

## Settings sections

```tsx
<ScrollArea>
  <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} padding={16} minWidth={400} maxWidth={640}>
    <PipelineSettingsPageGeneral />
    <PipelineSettingsPageSchedule />
    <PipelineSettingsPageNotifications />
    <PipelineSettingsPageAdvanced />
    <PipelineSettingsPageDanger />
  </Flex>
</ScrollArea>
```

- **The page composes flat sections.** Each section fetches the entity itself and returns `null` when it does not apply.
- **A section is a collapsible `Widget`**, `gap={12}` between widgets, all the same size. Only General starts open. A section that holds optional config starts open only when that config is in use (Schedule when the saved schedule is on, Notifications when a notifier exists), so it reads from data the page already suspended on. The create wizard's delivery step uses the same column of sections, starting with one section per connection (a node `header` with the `ConnectorTile` and name, the kind as `subheader`, a sink's write mode in `actions`, and its config fields as the collapsible body), then Schedule, Notifiers and Worker configuration.
- **Every section has local state applied by one Save.** Toggles included. No immediate-action RPCs on a settings page. Cancel is `SECONDARY` and disabled unless dirty, Save is `PRIMARY` with `isDisabled={!canSave}` and `isLoading`. See [forms.md](./forms.md).
- **Fields are always visible.** No "Add X" step for cheap config.
- **The danger zone** is a plain `Widget` with a space-between row, a title over a `BODY_SM` `SECONDARY` description, and an `ERROR` button with `TrashIcon`. It confirms through `useConfirm` and `ConfirmDialog` with `confirmValue` set to the entity's name and `isMatch={isPipelineNameMatch}` so a typed `->` matches the shown `→`.

## Drawer

- Opened by `?connectionId=` and rendered by `FilamentLayout`. It takes `connectionId`, `isOpen` and `onClose` as props and reads no search params. See [routing.md](./routing.md).
- Pass `hasDividers` so a hairline separates the title row from the scrolling body. DLS 2.10 defaults it to off.
- `renderContent()` checks `isError` (an `ErrorLayout` with a Close button), then a missing entity (`PendingLayout`), then the content.
- Actions on the thing the drawer shows go in `actions` as a ⋯ `Menu`, Delete last and red. A read-only drawer has no footer.
- Body blocks, shared with the canvas panel. `KeyValueList` with `KeyValueListRow`s (label on the left, a node on the right, hairline separators), a collapsible section `Widget` with a count `Chip` in `actions` (the chip hides a zero count) and `EmptyLayout size={EmptyLayoutSize.SMALL}` when empty, and a JSON section around `CodeBlock`.
- A related-entities list inside a drawer is a stack of compact `PipelineCard` rows, not a table.

## Modal

- Opened by `?flow=` and rendered by `FilamentLayout`. It stays mounted, takes `isOpen`, `onClose` and its record as props, and keys its provider with `useOverlaySession(isOpen)` so each open starts fresh while the exit still animates.
- `ModalSize.MEDIUM` for a form or a dialog, `X_LARGE` for a chooser or a multi-step wizard.
- The same shell shows every state. Error is an `ErrorLayout` with a Close button inside the body, loading is `PendingLayout`, loaded is the provider and the content.
- The footer is end-aligned, Back or Cancel `SECONDARY` first and the one `PRIMARY` action last. A pending action uses the button's `isLoading`.
- A wizard's sidebar is a DLS `Stepper`. While Next is disabled the footer shows a red "Invalid" `Chip` whose `tooltip` lists the blocking hints.

## Confirmation

- `useConfirm` (`hooks/useConfirm.ts`) holds the target, runs the mutation, toasts success and failure, closes, then calls `onConfirmed`.
- DLS `ConfirmDialog` with `isDestructive`, a question header ("Delete connection?"), a consequence description, a verb label and `confirmValue` for type-to-confirm. It consumes the promise `handleConfirm` returns, so there is no `isPending` plumbing. A caveat is an `Alert` passed as `children`.
- Gate a dialog on its host's `isOpen` too (`isOpen && confirmIsOpen`) so it closes with its drawer.
- Create forms and info dialogs are a plain `Modal`.

## State layouts

The three DLS state layouts fill their region. Each has its own size enum (`EmptyLayoutSize`, `ErrorLayoutSize`, `PendingLayoutSize`) exported from its module.

| | Use |
|---|---|
| `PendingLayout` | Whole-view loading. The route pending component, a drawer or modal body before its entity arrives, a form body before its schema arrives. |
| `EmptyLayout` | Nothing to show. `icon` for a plain state, `graphic` for a first-run illustration built from `components/EmptyGraphic`. |
| `ErrorLayout` | Something failed, including not found. Always red. `detail={IS_DEBUG ? error.message : undefined}`. |

- Content-shaped loading is a DLS `Skeleton` in a sized `Box`, for table cells, KPI numbers and inline names.
- An error branch gets `ErrorLayout`, never an `EmptyLayout` with a red icon.
- Panel sections use the `SMALL` size with a header and description.

## Status marks

- **Every run status is a square.** `PipelineRunStatusSwatch` (`components/runs/`) draws a DLS `Square` from `PIPELINE_RUN_STATUS_TO_HUE_MAP[status]`, a `RoleColor | undefined` where `undefined` leaves the neutral fill. The same map feeds chart series colours, and run-status charts pass `swatch={ChartSwatch.SQUARE}` so legends match.
- **One map owns each status colour.** Run status lives in `components/runs/constants.ts`, beside `PIPELINE_RUN_STATUS_TO_LABEL_MAP`. Add a status there and nowhere else.
- **`Beacon` is the live state of one thing**, never a run status. The connection form's "Connected" is one.
- **`Chip` is for small meta badges.** Status through `variant`, category through `color`. Pass `tooltip` on the Chip, never wrap it in `Tooltip`.
- Permission-gated actions are hidden or passed as `undefined`, not disabled. A page the viewer cannot use renders an `ErrorLayout` ("Admins only").

## Formatting and copy

DLS `utils/format` formats values and accepts `bigint` (`formatNumber`, `formatBytes`, `formatDuration`, `formatRelativeTime`, `formatDate`, `EMPTY_VALUE`). `utils/format.ts` keeps only Filament's rules, `formatTimestamp` (proto `0n` is `EMPTY_VALUE`) and `formatVersion`. Entity formatters live in the domain's `utils.ts` (`formatPipelineName`, `formatMemberName`). Counts with nouns go through `pluralize("sink", n, true)`.

- Sentence case everywhere. Buttons are a verb or verb plus noun ("New pipeline", "Run now", "Delete connection"). Never "OK", "Submit", "Yes".
- Placeholders end with three dots ("Enter connection name...").
- Toasts. Success is noun plus past tense ("Pipeline saved"), with a description that names the entity. Error is "<Verb> failed" with `getErrorMessage(error, fallback)`.
- Confirm dialogs ask a question with the entity and describe the consequence.
- Empty states say "No <things> yet" or "No <things>" with a sentence that says what to do. A filtered empty result says "No <things> match your search".
- Inline validation is short and imperative with no period ("Name is required").
- Missing values are `EMPTY_VALUE`. Arrows in summaries are `→`.
- Ids, emails, numbers, commands and log lines are mono. Error text in tooltips is mono and selectable.
