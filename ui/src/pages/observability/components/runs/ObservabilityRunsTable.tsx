import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  type Row,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { ListRunsRequestSchema, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import ObservabilityRunsTableColumnConnectors from "@/pages/observability/components/runs/columns/ObservabilityRunsTableColumnConnectors";
import { OBSERVABILITY_RUNS_TABLE_LIMIT } from "@/pages/observability/components/runs/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import { createTimeframeSince } from "@/pages/observability/utils";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { useListRunsQuery } from "@/api/queries/runs";

import {
  formatBytes,
  formatCount,
  formatDuration,
  formatSeconds,
  formatTimestamp,
} from "@/utils/format";

interface ObservabilityRunsTableProps {
  timeframe: ObservabilityTimeframe;
  statuses: RunStatus[];
}

const ObservabilityRunsTable = ({ timeframe, statuses }: ObservabilityRunsTableProps) => {
  const navigate = useNavigate();

  // Scheduled runs are upcoming — they have no started_at, so the timeframe
  // window can't apply to them. They're fetched unwindowed and pinned first.
  const includeScheduled = statuses.includes(RunStatus.SCHEDULED);
  const windowedStatuses = useMemo(
    () => statuses.filter((status) => status !== RunStatus.SCHEDULED),
    [statuses],
  );

  const input = useMemo(
    () =>
      create(ListRunsRequestSchema, {
        status: windowedStatuses,
        sinceMs: createTimeframeSince(timeframe),
        limit: OBSERVABILITY_RUNS_TABLE_LIMIT,
      }),
    [timeframe, windowedStatuses],
  );
  const scheduledInput = useMemo(
    () =>
      create(ListRunsRequestSchema, {
        status: [RunStatus.SCHEDULED],
        limit: OBSERVABILITY_RUNS_TABLE_LIMIT,
      }),
    [],
  );

  const { data, isLoading } = useListRunsQuery({
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
        size: 110,
        cellLoading: () => <TextShimmer width={64} height={18} />,
        cell: ({ row }) => (
          <PipelineHistoryRunStatus status={row.original.status} error={row.original.error} />
        ),
      },
      {
        id: "runId",
        header: "Run",
        size: 180,
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
        size: 140,
        cellLoading: () => <TextShimmer width={120} height={18} />,
        cell: ({ row }) => <ObservabilityRunsTableColumnConnectors runInfo={row.original} />,
      },
      {
        id: "startedAt",
        header: "Started",
        size: 160,
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
        size: 100,
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
        size: 100,
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
        size: 100,
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
        size: 100,
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
        size: 100,
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
  const windowedRuns = windowedStatuses.length ? (data?.runs ?? []) : [];
  // A just-promoted run can sit in the stale scheduled cache and the fresh
  // windowed result at once — the windowed row is the current truth.
  const windowedIds = new Set(windowedRuns.map((run) => run.runId));
  const runs = [...scheduledRuns.filter((run) => !windowedIds.has(run.runId)), ...windowedRuns];

  const handleRowClick = (row: Row<RunInfo>) => {
    navigate({
      to: "/pipelines/$id/history",
      params: {
        id: row.original.pipelineId,
      },
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
      contentWhenEmpty={
        <Text variant={TextVariant.TERTIARY}>No runs in the selected timeframe</Text>
      }
      maxHeight={640}
      fillWidth
      noLastRowPadding
      noLastRowBorder
    />
  );
};

export default ObservabilityRunsTable;
