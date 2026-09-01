import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  ColumnPin,
  type Row,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import ObservabilityRunsTableColumnFlow from "@/pages/observability/components/runs/columns/ObservabilityRunsTableColumnFlow";
import {
  OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP,
  OBSERVABILITY_RUNS_SCHEDULED_INPUT,
  OBSERVABILITY_RUNS_TABLE_COLUMN_ID_STARTED_AT,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME,
  OBSERVABILITY_RUNS_TABLE_EMPTY_STATE_HEIGHT,
  OBSERVABILITY_RUNS_TABLE_HEIGHT,
} from "@/pages/observability/components/runs/constants";
import {
  createObservabilityRunsSorting,
  createObservabilityRunsSortSearch,
  createRunsWindowInput,
  type ObservabilityRunsTableSortingChange,
} from "@/pages/observability/components/runs/utils";
import { ObservabilityRunsView, ObservabilityTimeframe } from "@/pages/observability/types";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { useListRunsInfiniteQuery, useListRunsQuery } from "@/api/queries/runs";
import { createListSortingInput } from "@/api/utils";

import {
  formatBytes,
  formatCount,
  formatDuration,
  formatSeconds,
  formatTimestamp,
} from "@/utils/format";

const ObservabilityRunsTable = () => {
  const navigate = useNavigate();
  const {
    runs: view = ObservabilityRunsView.PAST,
    timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
    runsBucket,
    runsStatus,
    sortBy,
    sortOrder,
  } = useSearch({ from: "/_app/_main/observability" });

  const windowedStatuses = useMemo(
    () => statuses.filter((status) => status !== RunStatus.SCHEDULED),
    [statuses],
  );

  const input = useMemo(
    () => ({
      status: runsStatus === undefined ? windowedStatuses : [runsStatus],
      ...createRunsWindowInput(timeframe, runsBucket),
      ...(sortBy === undefined ? {} : { sorting: createListSortingInput({ sortBy, sortOrder }) }),
    }),
    [timeframe, windowedStatuses, runsBucket, runsStatus, sortBy, sortOrder],
  );

  const { data, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useListRunsInfiniteQuery({
      input,
      options: {
        enabled: view === ObservabilityRunsView.PAST && windowedStatuses.length > 0,
      },
    });
  const { data: scheduledData, isLoading: isLoadingScheduled } = useListRunsQuery({
    input: OBSERVABILITY_RUNS_SCHEDULED_INPUT,
    options: { enabled: view === ObservabilityRunsView.UPCOMING },
  });

  const columns = useMemo<ColumnDef<RunInfo>[]>(() => {
    const baseColumns: ColumnDef<RunInfo>[] = [
      {
        id: "status",
        header: "Status",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
        pin: ColumnPin.LEFT,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={64} height={18} />,
        cell: ({ row }) => (
          <PipelineHistoryRunStatus status={row.original.status} error={row.original.error} />
        ),
      },
      {
        id: "flow",
        header: "Flow",
        minSize: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW,
        pin: ColumnPin.LEFT,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={120} height={18} />,
        cell: ({ row }) => <ObservabilityRunsTableColumnFlow runInfo={row.original} />,
      },
      {
        id: "pipeline",
        header: "Pipeline",
        minSize: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={120} height={14} />,
        cell: ({ row }) => <PipelineName pipelineId={row.original.pipelineId} />,
      },
    ];

    if (view === ObservabilityRunsView.UPCOMING) {
      return [
        ...baseColumns,
        {
          id: "scheduledAt",
          header: "Scheduled",
          size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
          align: ColumnAlign.RIGHT,
          enableSorting: false,
          cellLoading: () => <TextShimmer width={100} height={14} />,
          cell: ({ row }) => (
            <Text size={TextSize.BODY_SM} isEllipsis>
              {formatTimestamp(row.original.scheduledAt)}
            </Text>
          ),
        },
      ];
    }

    return [
      ...baseColumns,
      {
        id: OBSERVABILITY_RUNS_TABLE_COLUMN_ID_STARTED_AT,
        header: "Started",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
        accessorFn: (run) => Number(run.startedAt),
        enableSorting: true,
        sortDescFirst: true,
        cellLoading: () => <TextShimmer width={100} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isEllipsis>
            {formatTimestamp(row.original.startedAt)}
          </Text>
        ),
      },
      {
        id: "duration",
        header: "Duration",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={60} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isEllipsis>
            {formatDuration(row.original.startedAt, row.original.endedAt)}
          </Text>
        ),
      },
      {
        id: "records",
        header: "Records",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={48} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {formatCount(row.original.records)}
          </Text>
        ),
      },
      {
        id: "volume",
        header: "Volume",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={52} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {formatBytes(row.original.bytes)}
          </Text>
        ),
      },
      {
        id: "cpu",
        header: "CPU",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={48} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {row.original.cpuSeconds ? formatSeconds(row.original.cpuSeconds) : "—"}
          </Text>
        ),
      },
      {
        id: "memory",
        header: "Memory",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY,
        align: ColumnAlign.RIGHT,
        enableSorting: false,
        cellLoading: () => <TextShimmer width={52} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {row.original.memoryPeakBytes ? formatBytes(row.original.memoryPeakBytes) : "—"}
          </Text>
        ),
      },
    ];
  }, [view]);

  const scheduledRuns = useMemo(
    () => [...(scheduledData?.runs ?? [])].sort((a, b) => Number(a.scheduledAt - b.scheduledAt)),
    [scheduledData],
  );
  const windowedRuns = windowedStatuses.length
    ? (data?.pages.flatMap((page) => page.runs) ?? [])
    : [];
  const runs = view === ObservabilityRunsView.UPCOMING ? scheduledRuns : windowedRuns;

  const sorting = useMemo(
    () => createObservabilityRunsSorting({ sortBy, sortOrder }),
    [sortBy, sortOrder],
  );

  const handleSortingChange: ObservabilityRunsTableSortingChange = (updater) => {
    const next = typeof updater === "function" ? updater(sorting) : updater;
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({ ...prev, ...createObservabilityRunsSortSearch(next) }),
    });
  };

  const handleRowClick = (row: Row<RunInfo>) => {
    navigate({
      to: "/pipelines/$id/history",
      params: {
        id: row.original.pipelineId,
      },
      search: { runId: [row.original.id] },
    });
  };

  return (
    <InfiniteTable<RunInfo>
      columns={columns}
      data={runs}
      getRowId={(run) => run.id}
      onRowClick={handleRowClick}
      enableSorting
      manualSorting
      sorting={sorting}
      onSortingChange={handleSortingChange}
      isLoading={view === ObservabilityRunsView.UPCOMING ? isLoadingScheduled : isLoading}
      loadingRowCount={10}
      hasNextPage={view === ObservabilityRunsView.PAST && hasNextPage}
      isFetchingNextPage={isFetchingNextPage}
      fetchNextPage={fetchNextPage}
      contentWhenEmpty={
        <FlexWrapper
          height={OBSERVABILITY_RUNS_TABLE_EMPTY_STATE_HEIGHT}
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
        >
          <Text variant={TextVariant.TERTIARY}>
            {OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[view]}
          </Text>
        </FlexWrapper>
      }
      height={OBSERVABILITY_RUNS_TABLE_HEIGHT}
      fillWidth
    />
  );
};

export default ObservabilityRunsTable;
