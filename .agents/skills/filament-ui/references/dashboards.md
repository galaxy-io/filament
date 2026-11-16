# Dashboards and charts

The observability page (`pages/observability/`) is the reference dashboard. All of its view state is in the URL. The page suspends once, on the setup gate, the pipeline list and the connection lists, because the run wall, the side lists and the banner are its structure. Every metric panel after that runs a plain query and never suspends.

## Composition

```tsx
<PageLayout
  header={<GalaxyFilamentWordmark size={OBSERVABILITY_WORDMARK_SIZE} ariaLabel="Filament" />}
  actions={<>…<ObservabilityPageActions />, New pipeline…</>}
  banner={<ObservabilityFailuresBanner />}
>
  <ChartGroupProvider shouldShareTooltip>
    <ScrollArea ref={scrollRef} className={OBSERVABILITY_PAGE_SCROLL_CSS}>
      <ObservabilityLayout>
        <ObservabilityBand />
        <Divider />
        <Box padding={16} fillWidth>
          <ObservabilityBlock>
            <ObservabilityPipelinesWidget />
            <ObservabilityBlockSide>
              <ObservabilityScheduledWidget />
              <ObservabilityConnectionsWidget kind={ConnectorKind.SOURCE} />
              <ObservabilityConnectionsWidget kind={ConnectorKind.SINK} />
            </ObservabilityBlockSide>
          </ObservabilityBlock>
        </Box>
      </ObservabilityLayout>
      <Flex ref={takeoverRef} …>
        <ObservabilityRunsTableWidget />
      </Flex>
    </ScrollArea>
  </ChartGroupProvider>
</PageLayout>
```

- **Gate first.** `useObservabilitySetup()` suspends on three `pageSize: 1` inputs in `components/setup/constants.ts` (sources and sinks filtered by kind, pipelines including deleted). Until a source, a sink and a pipeline exist the page renders `ObservabilitySetupChecklist` alone, with no `PageLayout`, so no header row or page actions show.
- **The page is a DLS `PageLayout`.** `ObservabilityPageActions` fills `actions` with `ObservabilityTimeframeSwitcher` (a `ToggleInput` over `ObservabilityTimeframe`, labelled from `OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP` the way Alchemy labels its timeframes), `ObservabilityRefreshIntervalSelect` (a `SelectInput` over `ObservabilityRefreshInterval`) and an icon-only Refresh `Button` (`ariaLabel`, `tooltip`) that awaits `useRefreshObservabilityQueries()` and holds `isLoading` in local state until it resolves.
- **Scroll with `ScrollArea`**, never `overflow="auto"`.
- **The layout is container queries on `ObservabilityLayout`.** The block is the run wall beside a 360px stack, and stacks under `OBSERVABILITY_BLOCK_STACK_MAX_WIDTH`. The wall sets the height. It never scrolls inside. It pages `OBSERVABILITY_PIPELINES_LIMIT` rows at a time, labelled only "1–10 of 33", with previous and next buttons in its footer and the page in `pipelinesPage`. It keeps its height on a short last page, so the wheel always moves the page. `ObservabilityBlockSide` takes `contain: size` so its lists end level with the wall. Scheduled flexes, Sources and Sinks hold `OBSERVABILITY_BLOCK_CONNECTIONS_HEIGHT`, and each scrolls inside a `ScrollArea` sized so a longer list shows part of its next row. `OBSERVABILITY_BLOCK_MIN_HEIGHT` keeps the stack usable with few pipelines.
- **`useObservabilityPipelines` owns pipeline health.** It returns the entries ordered failing, then running, then most recent run, the failing subset for the banner, and `pipelinesById` for name lookups.
- **The run wall follows the timeframe.** One bar per bucket (24 hours, 7 or 30 days), from two `PIPELINE_ID` timeseries filtered to completed and failed. The track width is a CSS variable so the header and rows line up.
- **Every card is `<Widget isFlush gap={0} header="…" icon={…} actions={…}>`.**
- `ChartGroupProvider shouldShareTooltip` at the root syncs hover across the three band charts.

## View state in the URL

```ts
export const observabilitySearchSchema = z.object({
  timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
  refreshInterval: z.enum(ObservabilityRefreshInterval).optional().catch(undefined),
  runs: z.enum(ObservabilityRunsView).optional().catch(undefined),
  throughput: z.enum(ObservabilityThroughputView).optional().catch(undefined),
  throughputPivot: z.enum(MetricDimension).optional().catch(undefined),
  usage: z.enum(ObservabilityUsageView).optional().catch(undefined),
  usagePivot: z.enum(MetricDimension).optional().catch(undefined),
  statuses: z.array(z.enum(RunStatus)).optional().catch(undefined),
  pipelinesPage: z.coerce.number().int().positive().optional().catch(undefined),
});
```

- Timeframe, the refresh interval, the runs view, the Throughput and Usage metrics and grouping, the status filter and the run wall page are all search params. There is no context provider for page state, and a link reproduces the dashboard.
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
        refreshInterval: parsed.refreshInterval ?? OBSERVABILITY_DEFAULT_REFRESH_INTERVAL,
        runs: parsed.runs ?? OBSERVABILITY_DEFAULT_RUNS_VIEW,
        throughput: parsed.throughput ?? OBSERVABILITY_BAND_VIEW_KEY_TO_DEFAULT_MAP.throughput,
        throughputPivot:
          parsed.throughputPivot ?? OBSERVABILITY_BAND_PIVOT_KEY_TO_DEFAULT_MAP.throughputPivot,
        usage: parsed.usage ?? OBSERVABILITY_BAND_VIEW_KEY_TO_DEFAULT_MAP.usage,
        usagePivot: parsed.usagePivot ?? OBSERVABILITY_BAND_PIVOT_KEY_TO_DEFAULT_MAP.usagePivot,
        statuses: parsed.statuses ?? OBSERVABILITY_RUNS_DEFAULT_STATUSES,
        pipelinesPage: parsed.pipelinesPage ?? OBSERVABILITY_DEFAULT_PIPELINES_PAGE,
      };
    },
  });
```

- Writes go through `useFilamentSearchUpdate<ObservabilitySearch>()` with `(prev) => ({ ...prev, key })`. Paging the run wall uses `replace: true` and writes `undefined` for the first page. A grouping select writes `undefined` for its tile's default and `MetricDimension.UNSPECIFIED` when cleared, so a cleared Usage chart stays ungrouped although its default is Pipeline. A metric toggle writes `undefined` for its tile's default from `OBSERVABILITY_BAND_VIEW_KEY_TO_DEFAULT_MAP`.

## Loading, empty, error

- Every metric panel runs its own plain query and takes its loading flag straight from it. Panels query through `useObservabilityAggregateQuery` and `useObservabilityTimeseriesQuery`, which add the page's refresh interval, so no panel passes `refetchInterval` itself. Charts pass `isFetching` to `isLoading`, so the DLS pulses the plot over the data it already has on a refetch and an empty plot on first load. A band tile shows a `Skeleton` in place of its value, and a top three list and Scheduled show `Skeleton` rows, while their query loads. Loading never shows the empty copy. The past runs table passes `isLoading || isFetchingNextPage`.
- No `keepPreviousData`. A changed timeframe shows loading states, not the previous window's numbers.
- No `PendingLayout` inside a dashboard. It would blank the whole card.
- Gate loading UI on `isLoading`, which is false for a disabled query.
- Table empty states are `EmptyLayout size={EmptyLayoutSize.SMALL}` with copy from `OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[view]`. Charts get `[]` and draw empty axes.
- The block's widgets show `ObservabilityWidgetEmpty` when they have nothing to list: a small `EmptyLayout` with a header only and the widget's icon in a 20px tile (`OBSERVABILITY_EMPTY_TILE_SIZE`) passed as `graphic`, smaller than the DLS 24px tile. The run wall shows "No pipelines" in place of its header and rows, Scheduled "No scheduled runs" once its query has loaded, and Sources and Sinks "No sources" and "No sinks". The empty state sits outside the `ScrollArea`.

## Charts

DLS charts come from `@galaxy-io/dls/charts/*` and use the DLS's own d3 dependencies. Add no chart library.

| Import | Use |
|---|---|
| `charts/AreaChart` | records stacked by pipeline or status, the Throughput tile |
| `charts/LineChart` + `type LineChartLineDatum` | CPU or memory per pipeline or status, the Usage tile |
| `charts/BarChart` + `type BarChartGroupDatum` | counts per bucket stacked by status, the Runs tile |
| `charts/ChartGroupProvider` | shared hover across the band |
| `charts/types` | `ChartCurve`, `ChartSwatch`, `ChartSeriesStyles`, `ChartValueFormatter` |

```tsx
<Flex height={OBSERVABILITY_BAND_CHART_HEIGHT} fillWidth>
  <LineChart<string>
    series={{ [label]: { label, color } }}
    lines={lines}
    valueFormatter={valueFormatter}
    labelFormatter={labelFormatter}
    curve={ChartCurve.LINEAR}
    isLoading={isFetching}
    swatch={ChartSwatch.SQUARE}
    hasLegend={false}
  />
</Flex>
```

- **Size with the parent.** Charts take no `width`, `height` or `aspectRatio`. They fill a `Flex` with `fillWidth` and the fixed `OBSERVABILITY_BAND_CHART_HEIGHT` (136). DLS 2 charts always draw their axes, so a small chart is a short one, not an axis-free one.
- **Data shapes.** Line and area charts take `series: Record<key, { label, color }>` plus one datum per key `{ metric: key, showArea, points: [{ x, y }] }`. A bar chart takes `series: ChartSeriesStyles<Metric>` plus `groups: [{ label, bars: [{ metric, components: [{ key, label, value, color }] }] }]`, and several components on one bar stack. The bar metric is a string literal type in `band/types.ts` (`type ObservabilityBandRunsMetric = "runs"`).
- **Colours.** Ungrouped, Throughput and Usage each draw one series in the `color` of the tile's current metric in `OBSERVABILITY_BAND_VIEW_KEY_TO_CONFIG_MAP`. Grouped by pipeline, each pipeline takes its rank's colour from `OBSERVABILITY_BAND_PIPELINE_COLORS`, and `getObservabilityBandTopColors` hands the top three list the same colours, so a pipeline's line and its swatch match. Anything keyed by run status reads `PIPELINE_RUN_STATUS_TO_HUE_MAP`, the same `RoleColor` map that colours `PipelineRunStatusSwatch`, so swatches match everywhere.
- **Hue maps are typed `Record<RunStatus, RoleColor | undefined>`.** `undefined` means no colour (`UNSPECIFIED` only, cancelled runs are `violet`). Status series and bar components without a hue are dropped, never drawn with a fallback.
- **Run statuses use square swatches.** Pass `swatch={ChartSwatch.SQUARE}`.
- **Formatters.** `valueFormatter` receives a `number` (`formatNumber(value, { precision: 0 })`, `formatDuration`). An axis formatter must print zero, so each Usage view carries an `axisFormat` for the chart beside the `format` that shows `EMPTY_VALUE` in the tile. The x key is the bucket start in millis as a string (`formatBucketKey`), and `getBucketLabelFormatter(granularity)` turns it into `HH:00` or `MM/DD`. A day label reads the bucket midpoint, so a daylight saving shift never moves it to the previous date.
- **Legends.** `hasLegend={false}` in the band, where the tile names the series.

## The band

`ObservabilityBand` is a grid of three `ObservabilityBandTile`s split by hairline borders, with a `Divider` under it. Under `OBSERVABILITY_BAND_WRAP_MAX_WIDTH` it wraps, Runs over a full row and Throughput beside Usage, so the Throughput and Usage headers keep their toggle and select beside the label. A tile stacks the label with an info `Icon` tooltip and an optional `actions` slot, the value row and a short chart, then a full-width `Divider` and the top three list. Tiles are not links and do not switch anything else on the page.

- **Runs** carries `ObservabilityRunsViewToggle`, the same Past and Upcoming toggle History uses, on the shared `runs` param, so one switch moves both. `ObservabilityBandRunsTile` matches the view to a whole tile.
  - **Past** (`ObservabilityBandRunsPastTile`) shows the total and the non-zero featured statuses plus an "Other" count whose `InfoIcon` tooltip lists the rest. Counts that do not fit wrap onto a hidden second line. Its chart is every executed status, ignoring the History filter, and its list is "Most failures".
  - **Upcoming** (`ObservabilityBandRunsUpcomingTile`) reads `useObservabilityBandUpcoming`, the scheduled runs list bucketed forward from the current bucket over the timeframe. It shows the scheduled count, a bar per bucket and "Next up", one row per pipeline with a relative time and an absolute tooltip. The scheduler plans only each schedule's next run, so the bars count next fires, and the info tooltip says so.
  - Both draw `ObservabilityBandRunsChart`, a presentational `BarChart` that takes its `groups`.
- **Throughput and Usage** are one `ObservabilityBandMetricTile`, configured by `viewKey`, `pivotKey` and `isStacked`. Each shows one metric at a time, picked by a `ToggleInput` beside the grouping select in the header and kept in `throughput` (Records or Volume, Records by default) or `usage` (CPU or Memory, CPU by default). The value, its unit, the chart, the top list's title and empty copy and the colour all come from that metric's `ObservabilityBandViewConfig` in `OBSERVABILITY_BAND_VIEW_KEY_TO_CONFIG_MAP`. Usage values show `EMPTY_VALUE` until runs report usage.
- **Grouping.** Throughput and Usage carry an `ObservabilityBandPivotSelect` in the tile header, a clearable "Group by" of Pipeline or Status kept in `throughputPivot` and `usagePivot`. Throughput defaults to ungrouped and Usage to Pipeline (`OBSERVABILITY_BAND_PIVOT_KEY_TO_DEFAULT_MAP`). The tile draws `ObservabilityBandTimeseriesChart`, which queries the metric grouped by the pivot and maps the series with `mapObservabilityBandLines`. Throughput stacks areas because records and bytes add up. Usage draws lines because per-run averages do not.
- `useObservabilityMetricInputs` builds every metric request on the page from the timeframe in one `useMemo`. It returns the `bucketStartMs`, `sinceMs` and `labelFormatter` every panel shares, and two creators that every request goes through, each taking a metric, a grouping and optional statuses. `createAggregateInput(metric, groupBy?, statuses?)` serves the tile totals (ungrouped), the Runs counts (`STATUS`) and the top lists and pipeline colours (`PIPELINE_ID`). `createSeriesInput(metric, groupBy, statuses?)` serves the band charts and the run wall. A panel names its request at the call site, for example `createSeriesInput(Metric.RUN_COUNT, MetricDimension.PIPELINE_ID, [RunStatus.FAILED])`, and panels share identical requests through the cache. History reads `sinceMs` from the same hook.
- `useObservabilityBandRanking` sorts a `PIPELINE_ID` aggregate on the client for the top lists and the pipeline colours of the grouped charts. `ObservabilityBandTopList` feeds a ranking to `ObservabilityBandList`, which shows `OBSERVABILITY_BAND_TOP_COUNT` rows and links each name to its pipeline. Rankings keep values above zero for pipelines the page loaded, so a deleted pipeline never ranks.

## Runs

The `runs` param (`ObservabilityRunsView.PAST` or `UPCOMING`) and the `statuses` filter drive the History table only.

- **`ObservabilityRunsTableWidget`** titles itself from `OBSERVABILITY_RUNS_VIEW_TO_TABLE_HEADER_MAP[view]`, takes `ObservabilityRunsTableActions` as `actions` and renders `ObservabilityRunsPastTable` or `ObservabilityRunsUpcomingTable`.
- **`ObservabilityRunsTableActions`** holds `ObservabilityRunsViewToggle`, which writes `undefined` for Past, and the status `MultiSelectInput` (options prefixed with their swatch, `selectAllLabel={OBSERVABILITY_RUNS_ALL_STATUSES_LABEL}`, the collapsed value rendered as `pluralize("status", n, true)`). Upcoming disables the status select.
- **Shared columns.** `OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS` in `runs/columns/constants.tsx` holds status, flow and pipeline. Each table spreads it and appends its own columns as a private module constant (`OBSERVABILITY_RUNS_PAST_TABLE_COLUMNS`, `OBSERVABILITY_RUNS_UPCOMING_TABLE_COLUMNS`). Cells self-fetch by id (`PipelineName`, `PipelineFlow`).
- **Past table input.** The status filter (without `SCHEDULED`) and `sinceMs` from `createTimeframeSince(timeframe, bucketStartMs)`. The table has no sortable column, because the API cannot sort runs by start time and the default order already is.
- **Drill-down.** A row click deep-links to the run in history.

```ts
navigate({
  to: FilamentPath.PIPELINE_HISTORY,
  params: { id: row.pipelineId },
  search: { runId: [row.id] },
});
```

## Maps and constants

- Page-wide maps live in `pages/observability/constants.ts`. `OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP` gives each timeframe its `durationMs` and `granularity`, and `OBSERVABILITY_GRANULARITY_TO_DURATION_MS_MAP` gives each granularity its bucket length. `OBSERVABILITY_REFRESH_INTERVAL_TO_MS_MAP` gives each refresh interval the `refetchInterval` every metrics panel passes. Defaults (`OBSERVABILITY_DEFAULT_TIMEFRAME`, `OBSERVABILITY_DEFAULT_REFRESH_INTERVAL`, `OBSERVABILITY_DEFAULT_PIPELINES_PAGE`) sit beside them, with `OBSERVABILITY_PIPELINE_HEALTH_TO_RANK_MAP`. Bucket start, offset and label are functions in `utils.ts`.
- Card-level maps live in the card folder's `constants.ts` (`components/band/constants.ts`, `components/pipelines/constants.ts`, `components/runs/constants.ts`, `components/setup/constants.ts`).
- Enums live in `pages/observability/types.ts` (`ObservabilityTimeframe`, `ObservabilityRefreshInterval`, `ObservabilityRunsView`, `ObservabilityPipelineHealth`).
- Pure request builders live in `utils.ts` (`createTimeframeSince`, `createObservabilityTimeseriesInput`, `createObservabilityStatusFilter`). Every metric panel gets its request from `useObservabilityMetricInputs`. Band mappers live in `band/utils.ts` (`mapObservabilityBandRunsGroups`, `mapObservabilityBandPipelineLines`). The inputs take `bucketStartMs` from `useObservabilityBucketStart`, which moves at every hour or day boundary, so the window slides forward instead of freezing at mount.

## Setup checklist

`components/setup/ObservabilitySetupChecklist.tsx` is the onboarding screen shown before the dashboard has data. It fills the content area as the page's `main`, with the title as its `h1`. A `GridBackground` sits behind a centred column with the `GalaxyFilamentWordmark`, a card with an "n of total complete" caption, one `ObservabilitySetupChecklistStep` per step, and a `ProgressBar`. Every visual of a step row comes from status-keyed maps in `setup/constants.ts`, and a completed step shows `Chip label="Done" variant={ChipVariant.SUCCESS}`. Clicking a step opens the matching global flow through `OBSERVABILITY_SETUP_STEP_TO_FLOW_MAP` and `useFilamentFlowOpen`.
