# Dashboards and charts

The observability page (`pages/observability/`) is the reference dashboard. It differs from the rest of the app in two ways. Its panels do not suspend, and all of its view state is in the URL.

## Composition

```tsx
const ObservabilityPage: FC = () => {
  const { isComplete } = useObservabilitySetup();

  if (!isComplete) {
    return (
      <PageLayout header="Observability">
        <ObservabilitySetupChecklist />
      </PageLayout>
    );
  }

  return (
    <PageLayout header="Observability" actions={<ObservabilityPageActions />}>
      <ChartGroupProvider shouldShareTooltip>
        <ScrollArea>
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} padding={16} gap={12} fillWidth>
            <ObservabilityMetricsWidget />
            <Flex gap={12} alignItems={AlignItems.STRETCH} wrap={FlexWrap.WRAP} fillWidth>
              <FlexItem grow={1} basis={OBSERVABILITY_WIDGET_BASIS} minWidth={0}>
                <ObservabilityTimeseriesWidget
                  header="Throughput"
                  icon={GaugeIcon}
                  isStacked
                  views={OBSERVABILITY_THROUGHPUT_VIEWS}
                  viewToConfigMap={OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP}
                  defaultView={ObservabilityThroughputView.RECORDS}
                  viewSearchKey="throughput"
                  pivotSearchKey="throughputPivot"
                />
              </FlexItem>
              <FlexItem grow={1} basis={OBSERVABILITY_WIDGET_BASIS} minWidth={0}>
                <ObservabilityTimeseriesWidget header="Usage" icon={CpuIcon} … />
              </FlexItem>
            </Flex>
            <FlexItem fillWidth>
              <ObservabilityRunsChartWidget />
            </FlexItem>
            <FlexItem fillWidth>
              <ObservabilityRunsTableWidget />
            </FlexItem>
          </Flex>
        </ScrollArea>
      </ChartGroupProvider>
    </PageLayout>
  );
};
```

- **Gate first.** `useObservabilitySetup()` suspends on `OBSERVABILITY_CONNECTIONS_INPUT` and `OBSERVABILITY_PIPELINES_INPUT`. Until a source, a sink and a pipeline exist the page renders `ObservabilitySetupChecklist`. After that nothing suspends.
- **The page is a DLS `PageLayout`.** `ObservabilityPageActions` fills `actions` with `ObservabilityTimeframeSwitcher` (a `ToggleInput` over `ObservabilityTimeframe`) and an icon-only Refresh `Button` (`ariaLabel`, `tooltip`) that calls `useRefreshObservabilityQueries()`.
- **Scroll with `ScrollArea`**, never `overflow="auto"`.
- **The responsive grid is flex wrap**, not CSS grid. Each card is a `FlexItem grow={1} basis={OBSERVABILITY_WIDGET_BASIS} minWidth={0}` (`"400px"`) in a `FlexWrap.WRAP` row.
- **Every card is `<Widget isFlush gap={0} header="…" icon={…} actions={…}>`.** The view toggle and the pivot or status select sit in `actions`. The body is the chart in a `Flex` with `padding={[12, 16]}` and a fixed `height` constant.
- `ChartGroupProvider shouldShareTooltip` at the root syncs hover across every chart on the page.

## View state in the URL

```ts
export const observabilitySearchSchema = listSearchParamsSchema
  .pick({ sortBy: true, sortOrder: true })
  .extend({
    timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
    runs: z.enum(ObservabilityRunsView).optional().catch(undefined),
    throughput: z.enum(ObservabilityThroughputView).optional().catch(undefined),
    throughputPivot: z.enum(MetricDimension).optional().catch(undefined),
    usage: z.enum(ObservabilityUsageView).optional().catch(undefined),
    usagePivot: z.enum(MetricDimension).optional().catch(undefined),
    statuses: z.array(z.enum(RunStatus)).optional().catch(undefined),
    runsBucket: z.coerce.bigint().positive().optional().catch(undefined),
    runsStatus: z.enum(RunStatus).optional().catch(undefined),
  });
```

- Timeframe, the runs view, each card's view and pivot, the status filter, the selected bar and the table sort are all search params. There is no context provider for page state, and a link reproduces the dashboard.
- **Defaults are applied in the read hook, never in the schema.** `useObservabilitySearch` in `module/hooks.ts` parses and fills them in its `select`, so every widget reads a complete value.

```ts
export const useObservabilitySearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => {
      const parsed = observabilitySearchSchema.parse(search);
      return {
        ...parsed,
        timeframe: parsed.timeframe ?? OBSERVABILITY_DEFAULT_TIMEFRAME,
        runs: parsed.runs ?? OBSERVABILITY_DEFAULT_RUNS_VIEW,
        statuses: parsed.statuses ?? OBSERVABILITY_RUNS_DEFAULT_STATUSES,
      };
    },
  });
```

- Card-level defaults that differ per card (`defaultView`, `defaultPivot`) are props and apply at the read site, `search[viewSearchKey] ?? defaultView`.
- Writes go through `useFilamentSearchUpdate<ObservabilitySearch>()` with `(prev) => ({ ...prev, key })`. Sort changes use `replace: true`.
- "Explicitly no pivot" is stored as `MetricDimension.UNSPECIFIED`, so it differs from "use the default pivot".
- Changing a parent filter clears its dependents. Timeframe, the runs view and the status filter each clear `runsBucket` and `runsStatus`.

## Loading, empty, error

- Every panel runs its own plain query and takes `isLoading` straight from it. Charts and `BigNumber` take `isLoading` as a prop. The past runs table passes `isLoading || isFetchingNextPage`.
- No `keepPreviousData`. A changed timeframe shows loading states, not the previous window's numbers.
- No `PendingLayout` inside a dashboard. It would blank the whole card.
- A query that has nothing to ask is disabled (`options: { enabled: windowedStatuses.length > 0 }`) and the panel renders `[]`. Gate loading UI on `isLoading`, which is false for a disabled query.
- Table empty states are `EmptyLayout size={EmptyLayoutSize.SMALL}` with copy from `OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[view]`. Charts get `[]` and draw empty axes.

## Charts

DLS charts come from `@galaxy-io/dls/charts/*` and use the DLS's own d3 dependencies. Add no chart library.

| Import | Use |
|---|---|
| `charts/AreaChart` | stacked trends over time (`isStacked`), the Throughput card |
| `charts/LineChart` + `type LineChartLineDatum` | unstacked trends, `showArea` for a single unpivoted series, the Usage card |
| `charts/BarChart` + `type BarChartGroupDatum` | counts per bucket, stacked by status |
| `charts/BigNumber`, `charts/BigNumberGroup` | KPI tiles |
| `charts/ChartGroupProvider` | shared hover across the page |
| `charts/types` | `ChartSwatch`, `ChartPalette`, `ChartCurve`, `ChartSeriesStyles`, `ChartSelection`, `ChartSelectionInput`, `ChartValueFormatter` |

```tsx
<Flex
  alignItems={AlignItems.START}
  direction={FlexDirection.COLUMN}
  padding={[12, 16]}
  height={OBSERVABILITY_TIMESERIES_CHART_HEIGHT}
  fillWidth
>
  {isStacked ? (
    <AreaChart<string>
      series={series}
      areas={lines}
      curve={curve}
      valueFormatter={valueFormatter}
      labelFormatter={bucketLabelFormatter}
      isLoading={isLoading}
      swatch={ChartSwatch.SQUARE}
      hasLegend
      isFilterable
      isStacked
    />
  ) : (
    <LineChart<string> series={series} lines={lines} … />
  )}
</Flex>
```

- **Size with the parent.** Charts take no `width`, `height` or `aspectRatio`. They fill a `Flex` with `fillWidth` and a fixed height, `OBSERVABILITY_TIMESERIES_CHART_HEIGHT` and `OBSERVABILITY_RUNS_CHART_HEIGHT` (both 240).
- **Data shapes.** Line and area charts take `series: Record<key, { label, color }>` plus one datum per key `{ metric: key, showArea, points: [{ x, y }] }`. A bar chart takes `series: ChartSeriesStyles<Metric>` plus `groups: [{ label, bars: [{ metric, components: [{ key, label, value, color }] }] }]`, and several components on one bar stack. The bar metric is a string literal type in `runs/types.ts` (`type ObservabilityRunMetric = "runs"`).
- **Colours.** A view config pins one `ChartPalette` slot (Records `PURPLE`, Volume `TEAL`, CPU `ORANGE`, Memory `PINK`). A pivot by pipeline cycles `OBSERVABILITY_TIMESERIES_PIVOT_PALETTE[index % length]`. Anything keyed by run status reads `PIPELINE_RUN_STATUS_TO_HUE_MAP`, the same `RoleColor` map that colours `PipelineRunStatusSwatch`, so swatches match everywhere.
- **Hue maps are typed `Record<RunStatus, RoleColor | undefined>`.** `undefined` means no colour (`UNSPECIFIED`, `CANCELED`). Status series and bar components without a hue are dropped, never drawn with a fallback.
- **Run statuses use square swatches.** Pass `swatch={ChartSwatch.SQUARE}`.
- **Formatters.** `valueFormatter` receives a `number` and comes from the view config (`formatNumber(value, { precision: 0 })`, `formatBytes`, `formatDuration`). The x key is the bucket start in millis as a string (`formatBucketKey`), and `OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].formatBucketLabel` turns it into `HH:00` or `MM/DD`.
- **Filtering.** `isFilterable` lets the legend toggle series. When a click on a bar should filter something else, control `selection` and `onSelectionChange` and write the result to the URL through `mapChartSelectionToRunsFilter(next[next.length - 1])`, which yields `runsBucket` and `runsStatus`.
- **Legends.** `hasLegend` where more than one series can show.

## The generic timeseries widget

`ObservabilityTimeseriesWidget` drives both trend cards. It is generic over the search key it owns, so the view, the config map and the default all type-check against that key with no casts.

```tsx
type ObservabilityTimeseriesViewKey = "throughput" | "usage";

type ObservabilityTimeseriesView<TKey extends ObservabilityTimeseriesViewKey> = NonNullable<
  ObservabilitySearch[TKey]
>;

interface ObservabilityTimeseriesWidgetProps<TKey extends ObservabilityTimeseriesViewKey> {
  header: string;
  icon: Icon;
  isStacked?: boolean;
  views: ObservabilityTimeseriesView<TKey>[];
  viewToConfigMap: Record<ObservabilityTimeseriesView<TKey>, ObservabilityChartView>;
  defaultView: ObservabilityTimeseriesView<TKey>;
  defaultPivot?: MetricDimension;
  viewSearchKey: TKey;
  pivotSearchKey: "throughputPivot" | "usagePivot";
}
```

- `views` (`OBSERVABILITY_THROUGHPUT_VIEWS`) orders the `ToggleInput` options, and `viewToConfigMap[id].label` labels them.
- Each config map is an exhaustive `Record<ViewEnum, ObservabilityChartView>` in `components/timeseries/constants.ts`.

```ts
export const OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP: Record<
  ObservabilityThroughputView,
  ObservabilityChartView
> = {
  [ObservabilityThroughputView.RECORDS]: {
    label: "Records",
    seriesLabel: "Records",
    metric: Metric.RUN_RECORDS,
    color: ChartPalette.PURPLE,
    valueFormatter: (value) => formatNumber(value, { precision: 0 }),
  },
  …
};
```

- `ObservabilityPivotSelect` is a clearable `SelectInput` over `METRIC_DIMENSION_PIVOT_OPTIONS`. Clearing it writes `MetricDimension.UNSPECIFIED`.
- `ObservabilityTimeseriesChart` builds its request with `createObservabilityTimeseriesInput(timeframe, { metrics, groupBy })` in a `useMemo`, and labels a pipeline pivot from `useListPipelinesQuery({ input: OBSERVABILITY_PIPELINES_INPUT })`.

## KPI tiles

`ObservabilityMetricsWidget` is one DLS `BigNumberGroup` with a `primary` tile and the rest as children.

```tsx
<BigNumberGroup
  ariaLabel="Run metrics"
  variant={BigNumberGroupVariant.PRIMARY}
  hasBorder
  hasFadeEdges
  fillWidth
  primary={
    <BigNumber
      label="Total runs"
      value={formatNumber(totalRuns, { precision: 0 })}
      isLoading={isTotalsLoading}
    />
  }
>
  <BigNumber
    label="Total records"
    value={formatNumber(totalRecords, { precision: 0 })}
    suffix={<Icon component={RowsIcon} variant={IconVariant.TERTIARY} size={14} />}
    isLoading={isTotalsLoading}
  />
  …
</BigNumberGroup>
```

- Each `BigNumber` takes a string `value`, a `suffix` (a 14px `TERTIARY` `Icon` or a `PipelineRunStatusSwatch`) and `isLoading` from the query that feeds it.
- Totals and per-status counts are two `useQueryAggregateQuery` calls whose inputs are built in one `useMemo` keyed on timeframe. Grouped rows become a `Map` through `mapOptionIdToEnum(RunStatus, row.key)`.
- The featured statuses get one tile each, Scheduled counts `OBSERVABILITY_RUNS_SCHEDULED_INPUT`, and an "Other" tile's `InfoIcon` `Tooltip` lists the remaining statuses with swatches and counts.

## Runs widgets

Runs are two cards, a chart and a table, both switched by the `runs` param (`ObservabilityRunsView.PAST` or `UPCOMING`).

- **`ObservabilityRunsChartWidget`** holds the view `ToggleInput` and the status `MultiSelectInput` (options prefixed with their swatch, `selectAllLabel={OBSERVABILITY_RUNS_ALL_STATUSES_LABEL}`, the collapsed value rendered as `pluralize("status", n, true)`, in a `Box width={OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH}`). Past renders `ObservabilityRunsChart`, a selectable `BarChart` over `createRunCountTimeseriesInput(timeframe, statuses)`. Upcoming renders `ObservabilityRunsScheduledChart`, which buckets scheduled runs with `createScheduledRunsChartGroups` and disables the status select.
- **`ObservabilityRunsTableWidget`** titles itself from `OBSERVABILITY_RUNS_VIEW_TO_TABLE_HEADER_MAP[view]` and renders `ObservabilityRunsPastTable` or `ObservabilityRunsUpcomingTable`, each a DLS `InfiniteTable` in a `Box height={OBSERVABILITY_RUNS_TABLE_HEIGHT}`.
- **Shared columns.** `OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS` in `runs/columns/constants.tsx` holds status, flow and pipeline. Each table spreads it and appends its own columns as a private module constant (`OBSERVABILITY_RUNS_PAST_TABLE_COLUMNS`, `OBSERVABILITY_RUNS_UPCOMING_TABLE_COLUMNS`). Cells self-fetch by id (`PipelineName`, `PipelineFlow`).
- **Past table input.** The selected bar's status or the status filter (without `SCHEDULED`), a time window from `createRunsWindowInput(timeframe, runsBucket)`, and the sort through `createListSortingInput` when `sortBy` is set. Sort maps to the URL with `createTableSorting` and `createTableSortSearch` over `OBSERVABILITY_RUNS_SORT_BY_TO_COLUMN_ID_MAP`.
- **Drill-down.** A row click deep-links to the run in history.

```ts
navigate({
  to: FilamentPath.PIPELINE_HISTORY,
  params: { id: row.pipelineId },
  search: { runId: [row.id] },
});
```

## Maps and constants

- Page-wide maps live in `pages/observability/constants.ts`. `OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP` gives each timeframe its `durationMs`, `granularity` and `formatBucketLabel`, and the `OBSERVABILITY_GRANULARITY_TO_*_MAP` records give each granularity its bucket length, start and offset. Shared inputs and defaults (`OBSERVABILITY_CONNECTIONS_INPUT`, `OBSERVABILITY_DEFAULT_TIMEFRAME`) sit beside them.
- Card-level maps live in the card folder's `constants.ts` (`components/timeseries/constants.ts`, `components/runs/constants.ts`, `components/setup/constants.ts`).
- Enums live in `pages/observability/types.ts` (`ObservabilityTimeframe`, `ObservabilityRunsView`, `ObservabilityThroughputView`, `ObservabilityUsageView`).
- Pure request builders live in `utils.ts` (`createTimeframeSince`, `createObservabilityTimeseriesInput`, `createRunsWindowInput`, `mapTimeseriesToChartGroups`).

## Setup checklist

`components/setup/ObservabilitySetupChecklist.tsx` is the onboarding screen shown before the dashboard has data. A `GridBackground` sits behind a centred column with the `GalaxyFilamentWordmark`, a card with an "n of total complete" caption, one `ObservabilitySetupChecklistStep` per step, and a `ProgressBar`. Every visual of a step row comes from status-keyed maps in `setup/constants.ts`, and a completed step shows `Chip label="Done" variant={ChipVariant.SUCCESS}`. Clicking a step opens the matching global flow through `OBSERVABILITY_SETUP_STEP_TO_FLOW_MAP` and `useFilamentFlowOpen`.
