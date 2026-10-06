# Dashboards and charts

The observability page is the reference dashboard. Its rules differ from the rest of the app in two ways. It does not use suspense, and all of its view state is in the URL.

## Composition

```tsx
<ChartGroupProvider shouldShareTooltip>
  <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth height="100%">
    <ObservabilityToolbar />
    <FlexItem grow={0} shrink={0} fillWidth>
      <Divider />
    </FlexItem>
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} padding={12} gap={12} fillWidth height="100%" overflow="auto">
      <ObservabilityMetricsWidget />
      <Flex gap={12} alignItems={AlignItems.STRETCH} wrap={FlexWrap.WRAP} fillWidth>
        <FlexItem grow={1} basis={OBSERVABILITY_TIMESERIES_WIDGET_BASIS} minWidth={0}>
          <ObservabilityTimeseriesWidget views={OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP} defaultView={ObservabilityThroughputView.RECORDS} viewSearchKey="throughput" pivotSearchKey="throughputPivot" />
        </FlexItem>
        <FlexItem grow={1} basis={OBSERVABILITY_TIMESERIES_WIDGET_BASIS} minWidth={0}>
          <ObservabilityTimeseriesWidget views={OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP} … />
        </FlexItem>
      </Flex>
      <ObservabilityRunsWidget />
    </Flex>
  </Flex>
</ChartGroupProvider>
```

- **Gate first.** The page calls `useObservabilitySetup()` (two suspense queries) and renders `ObservabilitySetupChecklist` until a source, a sink and a pipeline exist. After that, nothing suspends.
- **Toolbar** is a DLS `Topbar` with the page title as its child and the timeframe `ToggleInput` plus an icon-only Refresh button (`ariaLabel`, `tooltip`) in `actions`; it draws its own bottom hairline. Refresh invalidates each family through the exported key factories called with no input.
- **Responsive grid is flex wrap**, not CSS grid. `FlexWrap.WRAP` with `FlexItem grow={1} basis={400} minWidth={0}` per card.
- **Every card is `<Widget isFlush gap={0} header="…" actions={…}>`** (title left, view toggles and pivot select in `actions`; the Widget draws the divider under its header row). Children are the chart in a padded `Flex` with a fixed `height` constant (240 for timeseries, 250 for runs), and for the runs card a `Divider` and a table.
- `ChartGroupProvider shouldShareTooltip` at the page root syncs hover across charts.

## View state in the URL

```ts
const searchParams = z.object({
  timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
  throughput: z.enum(ObservabilityThroughputView).optional().catch(undefined),
  throughputPivot: z.enum(MetricDimension).optional().catch(undefined),
  statuses: z.array(z.enum(RunStatus)).optional().catch(undefined),
  runsBucket: z.coerce.bigint().positive().optional().catch(undefined),
  runsStatus: z.enum(RunStatus).optional().catch(undefined),
  sortBy: z.enum(SortBy).optional().catch(undefined),
  sortOrder: z.enum(SortOrder).optional().catch(undefined),
});
```

- Timeframe, each card's view, each card's pivot, the status filter, the selected bar and the table sort are all search params. No context provider for page state. A link reproduces the dashboard.
- Defaults at the read site. `const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({ from: "/_app/_main/observability" })`.
- A generic card is pointed at its own keys, typed as a literal union prop (`viewSearchKey: "throughput" | "usage"`), and reads `search[viewSearchKey]`.
- "Explicitly no pivot" is stored as `MetricDimension.UNSPECIFIED` so it differs from "use the default".
- Changing a parent filter clears its dependents. Timeframe clears `runsBucket` and `runsStatus`. The status filter clears the selected bar.

## Loading, empty, error

- Charts and tables take `isLoading` straight from the query. Tables use `isLoading || isFetchingNextPage` when paging.
- KPI values render a `Skeleton` in a fixed-width `Box` while loading, through a tiny helper in the widget. `const totalValue = (value: string) => isTotalsLoading ? <Box width={48}><Skeleton size={SkeletonSize.LARGE} /></Box> : value`.
- No `keepPreviousData`. A changed timeframe shows skeletons, not the previous window's numbers.
- No `PendingLayout` inside a dashboard. It would blank the whole card.
- Table empty states are a tertiary `Text` centred at a constant height, copy from an enum-keyed map (`OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[view]`). Charts get `[]` and draw empty axes.
- Queries switch off by view with `options: { enabled: view === ObservabilityRunsView.PAST && statuses.length > 0 }`. Gate loading UI on `isLoading`, which is false for a disabled query.

## Charts

DLS charts come from `@galaxy-io/dls/charts/*` and use the DLS's own d3 dependencies. No new chart library.

| Import | Use |
|---|---|
| `charts/LineChart` + `type LineChartLineDatum` | trends over time, `showArea` for a single unpivoted series |
| `charts/BarChart` + `type BarChartGroupDatum` | counts per bucket, stacked by status |
| `charts/StatChart` + `StatChartVariant` | a KPI (`value`, `trailing` for an icon or a status swatch, `isLoading`, `hasBorder`) |
| `charts/ChartGroupProvider` | shared hover across the page |
| `charts/types` | `ChartSwatch`, `ChartPalette`, `ChartCurve`, `ChartSeriesStyles`, `ChartSelection`, `ChartSelectionInput`, `ChartValueFormatter` |

```tsx
<LineChart<string>
  series={series}
  lines={lines}
  curve={curve}
  valueFormatter={valueFormatter}
  labelFormatter={bucketLabelFormatter}
  isLoading={isLoading}
  swatch={ChartSwatch.SQUARE}
  hasLegend
  isFilterable
/>

<BarChart
  series={OBSERVABILITY_RUNS_SERIES}
  groups={groups}
  labelFormatter={bucketLabelFormatter}
  selection={selection}
  onSelectionChange={handleSelectionChange}
  isFilterable
  isLoading={isLoading}
  minSegmentLength={OBSERVABILITY_RUNS_CHART_MIN_SEGMENT_LENGTH}
/>
```

- **Data shapes.** A line chart is `series: Record<key, { label, color }>` plus `lines: [{ metric: key, showArea, points: [{ x, y }] }]`. A bar chart is `series: ChartSeriesStyles<Metric>` plus `groups: [{ label, bars: [{ metric, components: [{ key, label, value, color }] }] }]`. Several components on one bar stack. The metric key type is a string literal type in the folder's `types.ts` (`type ObservabilityRunMetric = "runs"`).
- **Size with the parent.** No `width`, `height` or `aspectRatio` props. The chart fills a `Flex` with `fillWidth` and a fixed `height` constant. (`aspectRatio` derives height from a measured width that is 0 on the first frame and jumps.)
- **Colors are `ChartPalette` slots**, never hex. A view config pins one color (Records `PURPLE`, Volume `TEAL`, CPU `ORANGE`, Memory `PINK`). A pivot by pipeline cycles `OBSERVABILITY_TIMESERIES_PIVOT_PALETTE[index % length]`. A pivot by run status uses `PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP`, derived from the same hue map that draws every status square, so swatches match everywhere.
- **Run statuses use square swatches.** `swatch={ChartSwatch.SQUARE}` on any chart whose series are run statuses, matching `PipelineRunStatusSwatch`.
- **Formatters.** `valueFormatter` receives a `number`, so wrap the `bigint` helpers, `(value) => formatCount(BigInt(Math.round(value)))`. The x key is the bucket start in millis as a string (`formatBucketKey`), and `useBucketLabelFormatter(timeframe)` turns it into `HH:00` or `MM/DD` from `OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP`, which also holds each timeframe's `durationMs` and `granularity`. Integer series need a formatter that rounds, or fractional ticks render as duplicates.
- **Filtering.** `isFilterable` lets the legend toggle series. When clicking a bar should filter something else on the page, control `selection` + `onSelectionChange` and write the result to the URL (`mapChartSelectionToRunsFilter(next[next.length - 1])` → `runsBucket`, `runsStatus`).
- **Legends.** `hasLegend` where more than one series shows. Hide it in a small embed.
- **Sparkline in a table cell** (`PipelinesTableColumnRecentRuns`). The chart draws axes unconditionally, so suppress them with a fixed `width`/`height` (about 104×24), `margin` all zero (`left: 0` is what removes value ticks), `labelFormatter={() => ""}`, `showGrid={false}`, `showLegend={false}` and `noTooltip` so clicks reach `onRowClick`. A `value === 0` draws nothing, so clamp with `Math.max(1, value)` when a bar must stay visible.

## View configs

Each card is driven by an exhaustive `Record<ViewEnum, ObservabilityChartView>` in `pages/observability/components/timeseries/constants.ts`.

```ts
export const OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP: Record<ObservabilityThroughputView, ObservabilityChartView> = {
  [ObservabilityThroughputView.RECORDS]: {
    label: "Records",
    seriesLabel: "Records",
    metric: Metric.RUN_RECORDS,
    color: ChartPalette.PURPLE,
    valueFormatter: (value) => formatCount(BigInt(Math.round(value))),
  },
  …
};
```

The card's `ToggleInput` options are `Object.values(ViewEnum)` mapped through the config's `label`. The pivot `SelectInput` lists `MetricDimension` members.

## KPI tiles

- KPI tiles are DLS `StatChart`s used directly: a string `value`, `trailing` for the 14px tertiary `Icon` or a `PipelineRunStatusSwatch`, `isLoading` per query, `variant` and `hasBorder`.
- `MetricGroup` (`components/metrics/MetricGroup.tsx`) is one flush `Widget` row. An optional `primary` tile grows, the rest scroll horizontally.
- The observability widget puts "Total runs" as the primary tile, totals with icons at `TERTIARY`, one tile per status with its `PipelineRunStatusSwatch`, and an "Other" tile whose `InfoIcon` tooltip lists the remaining statuses with swatches and counts.
- Aggregates come from `useQueryAggregateQuery` with the input built in one `useMemo` keyed on timeframe. Grouped rows become `new Map(rows.map((row) => [Number(row.key) as RunStatus, row.values[0] ?? 0]))`.

## Filters and drill-down

- The status filter is a `MultiSelectInput` with a pinned "All statuses" option through `src/utils/select.ts` (`getSelectAllOptions`, `getSelectAllValue`, `getSelectAllChange`), each option prefixed with its swatch, the collapsed value rendered as `pluralize("status", n, true)`, in a `Box width={160}`. It writes `statuses` as `RunStatus[]` and clears the bar selection.
- The runs table combines the selected statuses or the selected bar's status with a time window from `createRunsWindowInput(timeframe, runsBucket)`. Row click deep-links to history with the run expanded. `navigate({ to: "/pipelines/$id/history", params: { id: row.pipelineId }, search: { runId: [row.id] } })`.
- A dashboard table that is also sortable maps its sort to the URL the same way the list pages do (`createObservabilityRunsSorting`, `createObservabilityRunsSortSearch` in `runs/utils.ts`).

## Setup checklist

`pages/observability/components/setup/ObservabilitySetupChecklist.tsx` is the onboarding screen shown before the dashboard has data. A `GridBackground` behind a centred column (wordmark, heading, subheading), a card with an "n of total complete" caption and one row per step, and a `ProgressBar variant={SUCCESS}`. Every visual of a step row comes from status-keyed maps in `setup/constants.ts` (icon, icon weight, icon variant, text variants, button variant, row opacity), and a completed step shows `Chip label="Done" variant={SUCCESS}`. Clicking a step opens the matching global flow (`Flow.CREATE_CONNECTION` with a `connectorKind`, or `Flow.CREATE_PIPELINE`).
