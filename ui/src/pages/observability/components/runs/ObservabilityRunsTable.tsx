import { useMemo } from "react";

import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import type { ObservabilityMockRunInfo } from "@/pages/observability/components/runs/types";
import { createMockRuns } from "@/pages/observability/components/runs/utils";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/utils/format";

const OBSERVABILITY_RUNS_TABLE_COLUMNS: ColumnDef<ObservabilityMockRunInfo>[] = [
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
        {row.original.pipelineName}
      </Text>
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
];

interface ObservabilityRunsTableProps {
  statuses: RunStatus[];
}

const ObservabilityRunsTable = ({ statuses }: ObservabilityRunsTableProps) => {
  const runs = useMemo(() => createMockRuns(), []);

  const filteredRuns = useMemo(
    () => runs.filter((run) => statuses.includes(run.status)),
    [runs, statuses],
  );

  return (
    <InfiniteTable<ObservabilityMockRunInfo>
      columns={OBSERVABILITY_RUNS_TABLE_COLUMNS}
      data={filteredRuns}
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
