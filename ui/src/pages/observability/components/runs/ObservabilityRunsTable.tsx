import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily, Side } from "@galaxy-io/dls/theme/enums";

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

  const columns = useMemo<TableColumn<RunInfo>[]>(() => {
    const baseColumns: TableColumn<RunInfo>[] = [
      {
        id: "status",
        header: "Status",
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
        pin: Side.LEFT,
        canSort: false,
        cell: ({ row }) => (
          <PipelineHistoryRunStatus
            status={row.status}
            error={row.error}
            executionStatus={row.executionStatus}
          />
        ),
      },
      {
        id: "flow",
        header: "Flow",
        minWidth: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW,
        pin: Side.LEFT,
        canSort: false,
        cell: ({ row }) => <ObservabilityRunsTableColumnFlow runInfo={row} />,
      },
      {
        id: "pipeline",
        header: "Pipeline",
        minWidth: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE,
        canSort: false,
        cell: ({ row }) => <PipelineName pipelineId={row.pipelineId} />,
      },
    ];

    if (view === ObservabilityRunsView.UPCOMING) {
      return [
        ...baseColumns,
        {
          id: "scheduledAt",
          header: "Scheduled",
          width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
          align: "right",
          cell: ({ row }) => (
            <Text size={TextSize.BODY_SM} lineClamp={1}>
              {formatTimestamp(row.scheduledAt)}
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
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
        accessor: (run) => Number(run.startedAt),
        canSort: true,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} lineClamp={1}>
            {formatTimestamp(row.startedAt)}
          </Text>
        ),
      },
      {
        id: "duration",
        header: "Duration",
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} lineClamp={1}>
            {formatDuration(row.startedAt, row.endedAt)}
          </Text>
        ),
      },
      {
        id: "records",
        header: "Records",
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {formatCount(row.records)}
          </Text>
        ),
      },
      {
        id: "volume",
        header: "Volume",
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {formatBytes(row.bytes)}
          </Text>
        ),
      },
      {
        id: "cpu",
        header: "CPU",
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {row.cpuSeconds ? formatSeconds(row.cpuSeconds) : "—"}
          </Text>
        ),
      },
      {
        id: "memory",
        header: "Memory",
        width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY,
        align: "right",
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {row.memoryPeakBytes ? formatBytes(row.memoryPeakBytes) : "—"}
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

  const handleSortingChange: ObservabilityRunsTableSortingChange = (next) => {
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({ ...prev, ...createObservabilityRunsSortSearch(next) }),
    });
  };

  const handleRowClick = (row: RunInfo) => {
    navigate({
      to: "/pipelines/$id/history",
      params: {
        id: row.pipelineId,
      },
      search: { runId: [row.id] },
    });
  };

  return (
    <Box height={OBSERVABILITY_RUNS_TABLE_HEIGHT}>
      <InfiniteTable<RunInfo>
        columns={columns}
        data={runs}
        getRowId={(run) => run.id}
        onRowClick={handleRowClick}
        sort={sorting}
        onSortChange={handleSortingChange}
        isLoading={
          view === ObservabilityRunsView.UPCOMING
            ? isLoadingScheduled
            : isLoading || isFetchingNextPage
        }
        onEndReached={() => {
          if (view === ObservabilityRunsView.PAST && hasNextPage) fetchNextPage();
        }}
        emptyState={
          <Flex
            height={OBSERVABILITY_RUNS_TABLE_EMPTY_STATE_HEIGHT}
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Text variant={TextVariant.TERTIARY}>
              {OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[view]}
            </Text>
          </Flex>
        }
      />
    </Box>
  );
};

export default ObservabilityRunsTable;
