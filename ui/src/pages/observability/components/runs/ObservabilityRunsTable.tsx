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

import { ListRunsRequestSchema, type RunInfo, type RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import { OBSERVABILITY_RUNS_TABLE_LIMIT } from "@/pages/observability/components/runs/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import { createTimeframeSince } from "@/pages/observability/utils";
import PipelineFlow, { PipelineFlowSize } from "@/pages/pipelines/components/flow/PipelineFlow";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { useListConnectionsQuery } from "@/api/queries/connections";
import { useListRunsQuery } from "@/api/queries/runs";

import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/utils/format";

interface ObservabilityRunsTableProps {
  timeframe: ObservabilityTimeframe;
  statuses: RunStatus[];
}

const ObservabilityRunsTable = ({ timeframe, statuses }: ObservabilityRunsTableProps) => {
  const navigate = useNavigate();

  const input = useMemo(
    () =>
      create(ListRunsRequestSchema, {
        status: statuses,
        sinceMs: createTimeframeSince(timeframe),
        limit: OBSERVABILITY_RUNS_TABLE_LIMIT,
      }),
    [timeframe, statuses],
  );

  const { data, isLoading } = useListRunsQuery({ input });

  const { data: connectionsData } = useListConnectionsQuery();

  const connectorsByConnectionId = useMemo(
    () =>
      new Map(
        (connectionsData?.connections ?? []).map((connection) => [
          connection.id,
          connection.connector,
        ]),
      ),
    [connectionsData],
  );

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
        cell: ({ row }) => (
          <PipelineFlow
            source={{
              connectionId: row.original.sourceConnectionId,
              connector:
                connectorsByConnectionId.get(row.original.sourceConnectionId) ??
                row.original.sourceConnectionId,
            }}
            sinks={
              row.original.sinkConnectionId
                ? [
                    {
                      connectionId: row.original.sinkConnectionId,
                      connector:
                        connectorsByConnectionId.get(row.original.sinkConnectionId) ??
                        row.original.sinkConnectionId,
                    },
                  ]
                : []
            }
            size={PipelineFlowSize.SMALL}
          />
        ),
      },
      {
        id: "startedAt",
        header: "Started",
        size: 140,
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
        align: ColumnAlign.RIGHT,
        accessorFn: (run) => Number(run.bytes),
        enableSorting: true,
        cellLoading: () => <TextShimmer width={52} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {formatBytes(row.original.bytes)}
          </Text>
        ),
      },
    ],
    [connectorsByConnectionId],
  );

  const runs = statuses.length ? (data?.runs ?? []) : [];

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
      isLoading={isLoading}
      contentWhenEmpty={
        <Text variant={TextVariant.TERTIARY}>No runs in the selected timeframe</Text>
      }
      fillWidth
      height={300}
    />
  );
};

export default ObservabilityRunsTable;
