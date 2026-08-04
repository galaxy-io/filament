import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { ListRunsRequestSchema, type RunInfo, type RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_RUNS_TABLE_LIMIT } from "@/pages/observability/components/runs/constants";
import PipelineFlow, { PipelineFlowSize } from "@/pages/pipelines/components/flow/PipelineFlow";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useSuspenseListPipelinesQuery } from "@/api/queries/pipelines";
import { useSuspenseListRunsQuery } from "@/api/queries/runs";

import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/utils/format";

interface ObservabilityRunsTableProps {
  statuses: RunStatus[];
}

const ObservabilityRunsTable = ({ statuses }: ObservabilityRunsTableProps) => {
  const { data } = useSuspenseListRunsQuery({
    input: create(ListRunsRequestSchema, {
      status: statuses,
      limit: OBSERVABILITY_RUNS_TABLE_LIMIT,
    }),
  });

  const { data: pipelinesData } = useSuspenseListPipelinesQuery();
  const { data: connectionsData } = useSuspenseListConnectionsQuery();

  const connectorsByConnectionId = useMemo(
    () =>
      new Map(
        connectionsData.connections.map((connection) => [connection.id, connection.connector]),
      ),
    [connectionsData.connections],
  );

  const pipelineNamesByPipelineId = useMemo(
    () =>
      new Map(
        pipelinesData.pipelines.map((pipeline) => [pipeline.id, formatPipelineName(pipeline)]),
      ),
    [pipelinesData.pipelines],
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
        id: "pipeline",
        header: "Pipeline",
        cellLoading: () => <TextShimmer width={120} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isEllipsis>
            {pipelineNamesByPipelineId.get(row.original.pipelineId) ?? row.original.pipelineId}
          </Text>
        ),
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
        size: 90,
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
        cellLoading: () => <TextShimmer width={52} height={14} />,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} isMonospace>
            {formatBytes(row.original.bytes)}
          </Text>
        ),
      },
    ],
    [pipelineNamesByPipelineId, connectorsByConnectionId],
  );

  const runs = statuses.length ? data.runs : [];

  return (
    <InfiniteTable<RunInfo>
      columns={columns}
      data={runs}
      getRowId={(run) => run.runId}
      contentWhenEmpty={
        <Text variant={TextVariant.TERTIARY}>No runs in the selected timeframe</Text>
      }
      fillWidth
      height={300}
    />
  );
};

export default ObservabilityRunsTable;
