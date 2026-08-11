import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useNavigate, useSearch } from "@tanstack/react-router";

import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  type Row,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { ListRunsRequestSchema, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import ObservabilityRunsTableColumnConnectors from "@/pages/observability/components/runs/columns/ObservabilityRunsTableColumnConnectors";
import {
  OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CONNECTORS,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RUN,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME,
  OBSERVABILITY_RUNS_TABLE_LIMIT,
  OBSERVABILITY_RUNS_TABLE_MAX_HEIGHT,
} from "@/pages/observability/components/runs/constants";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { createTimeframeSince } from "@/pages/observability/utils";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { useListRunsInfiniteQuery, useListRunsQuery } from "@/api/queries/runs";

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
    timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  } = useSearch({ from: "/_main/observability" });

  const includeScheduled = statuses.includes(RunStatus.SCHEDULED);
  const windowedStatuses = useMemo(
    () => statuses.filter((status) => status !== RunStatus.SCHEDULED),
    [statuses],
  );

  const input = useMemo(
    () => ({
      status: windowedStatuses,
      sinceMs: createTimeframeSince(timeframe),
    }),
    [timeframe, windowedStatuses],
  );
  const scheduledInput = useMemo(
    () =>
      create(ListRunsRequestSchema, {
        status: [RunStatus.SCHEDULED],
        pagination: create(PaginationRequestSchema, { total: OBSERVABILITY_RUNS_TABLE_LIMIT }),
      }),
    [],
  );

  const { data, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useListRunsInfiniteQuery({
      input,
      options: { enabled: windowedStatuses.length > 0 },
    });
  const { data: scheduledData, isLoading: isLoadingScheduled } = useListRunsQuery({
    input: scheduledInput,
    options: { enabled: includeScheduled },
  });

  const columns = useMemo<ColumnDef<RunInfo>[]>(
    () => [
      {
        id: "status",
        header: "Status",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
        cellLoading: () => <TextShimmer width={64} height={18} />,
        cell: ({ row }) => (
          <PipelineHistoryRunStatus status={row.original.status} error={row.original.error} />
        ),
      },
      {
        id: "runId",
        header: "Run",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RUN,
        cellLoading: () => <TextShimmer width={64} height={18} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isMonospace>
            {row.original.runId}
          </Text>
        ),
      },
      {
        id: "pipeline",
        header: "Pipeline",
        cellLoading: () => <TextShimmer width={120} height={14} />,
        cell: ({ row }) => <PipelineName pipelineId={row.original.pipelineId} />,
      },
      {
        id: "connectors",
        header: "Connectors",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CONNECTORS,
        cellLoading: () => <TextShimmer width={120} height={18} />,
        cell: ({ row }) => <ObservabilityRunsTableColumnConnectors runInfo={row.original} />,
      },
      {
        id: "startedAt",
        header: "Started",
        size: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
        accessorFn: (run) => Number(run.startedAt),
        enableSorting: true,
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
        accessorFn: (run) =>
          run.startedAt && run.endedAt ? Number(run.endedAt - run.startedAt) : -1,
        enableSorting: true,
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
        accessorFn: (run) => Number(run.records),
        enableSorting: true,
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
        accessorFn: (run) => Number(run.bytes),
        enableSorting: true,
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
        accessorFn: (run) => run.cpuSeconds,
        enableSorting: true,
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
        accessorFn: (run) => Number(run.memoryPeakBytes),
        enableSorting: true,
        cellLoading: () => <TextShimmer width={52} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {row.original.memoryPeakBytes ? formatBytes(row.original.memoryPeakBytes) : "—"}
          </Text>
        ),
      },
    ],
    [],
  );

  const scheduledRuns = includeScheduled ? (scheduledData?.runs ?? []) : [];
  const windowedRuns = windowedStatuses.length
    ? (data?.pages.flatMap((page) => page.runs) ?? [])
    : [];

  const windowedIds = new Set(windowedRuns.map((run) => run.runId));
  const runs = [...scheduledRuns.filter((run) => !windowedIds.has(run.runId)), ...windowedRuns];

  const handleRowClick = (row: Row<RunInfo>) => {
    navigate({
      to: "/pipelines/$id/history",
      params: {
        id: row.original.pipelineId,
      },
      search: { runId: row.original.runId },
    });
  };

  return (
    <InfiniteTable<RunInfo>
      columns={columns}
      data={runs}
      getRowId={(run) => run.runId}
      onRowClick={handleRowClick}
      enableSorting
      isLoading={isLoading || (includeScheduled && isLoadingScheduled)}
      loadingRowCount={1}
      hasNextPage={hasNextPage}
      isFetchingNextPage={isFetchingNextPage}
      fetchNextPage={fetchNextPage}
      contentWhenEmpty={
        <Text variant={TextVariant.TERTIARY}>No runs in the selected timeframe</Text>
      }
      maxHeight={OBSERVABILITY_RUNS_TABLE_MAX_HEIGHT}
      fillWidth
      noLastRowPadding
      noLastRowBorder
    />
  );
};

export default ObservabilityRunsTable;
