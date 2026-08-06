import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  type Row,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import {
  PIPELINES_TABLE_COLUMN_MIN_WIDTH_CONNECTORS,
  PIPELINES_TABLE_COLUMN_MAX_WIDTH_CONNECTORS,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
  PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
} from "@/pages/pipelines/components/table/constants";
import PipelinesTableFlowCell from "@/pages/pipelines/components/table/PipelinesTableFlowCell";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { formatBytes, formatDuration, formatTimeAgo } from "@/utils/format";

const PipelinesTableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const PIPELINES_TABLE_COLUMNS: ColumnDef<Pipeline>[] = [
  {
    id: "name",
    header: "Name",
    accessorFn: (pipeline) => formatPipelineName(pipeline),
    enableSorting: true,
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isEllipsis>
        {formatPipelineName(row.original)}
      </Text>
    ),
  },
  {
    id: "connectors",
    header: "Connectors",
    minSize: PIPELINES_TABLE_COLUMN_MIN_WIDTH_CONNECTORS,
    maxSize: PIPELINES_TABLE_COLUMN_MAX_WIDTH_CONNECTORS,
    cellLoading: () => <TextShimmer width={120} height={18} />,
    cell: ({ row }) => <PipelinesTableFlowCell pipeline={row.original} />,
  },
  {
    id: "status",
    header: "Status",
    size: PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
    accessorFn: (pipeline) => pipeline.lastRunStatus,
    enableSorting: true,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) =>
      row.original.lastRunAt > 0n ? (
        <PipelineHistoryRunStatus status={row.original.lastRunStatus} />
      ) : (
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          Never run
        </Text>
      ),
  },
  {
    id: "lastRun",
    header: "Last run",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
    accessorFn: (pipeline) => Number(pipeline.lastRunAt),
    enableSorting: true,
    cellLoading: () => <TextShimmer width={64} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {row.original.lastRunAt > 0n ? formatTimeAgo(row.original.lastRunAt) : "—"}
      </Text>
    ),
  },
  {
    id: "lastDuration",
    header: "Duration",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
    accessorFn: (pipeline) =>
      pipeline.lastRunAt && pipeline.lastRunEndedAt
        ? Number(pipeline.lastRunEndedAt - pipeline.lastRunAt)
        : -1,
    enableSorting: true,
    cellLoading: () => <TextShimmer width={48} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatDuration(row.original.lastRunAt, row.original.lastRunEndedAt)}
      </Text>
    ),
  },
  {
    id: "lastVolume",
    header: "Volume",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
    align: ColumnAlign.RIGHT,
    accessorFn: (pipeline) => Number(pipeline.lastRunBytes),
    enableSorting: true,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.lastRunAt > 0n ? formatBytes(row.original.lastRunBytes) : "—"}
      </Text>
    ),
  },
];

interface PipelinesTableProps {
  pipelines: Pipeline[];
}

const PipelinesTable = ({ pipelines }: PipelinesTableProps) => {
  const navigate = useNavigate();

  const handleRowClick = (row: Row<Pipeline>) => {
    navigate({
      to: "/pipelines/$id",
      params: { id: row.original.id },
    });
  };

  return (
    <PipelinesTableWrapper>
      <InfiniteTable<Pipeline>
        columns={PIPELINES_TABLE_COLUMNS}
        data={pipelines}
        getRowId={(pipeline) => pipeline.id}
        contentWhenEmpty={
          <EmptyLayout
            icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
            message="No pipelines match your search"
          />
        }
        onRowClick={handleRowClick}
        enableSorting
        fillWidth
        fillHeight
      />
    </PipelinesTableWrapper>
  );
};

export default PipelinesTable;
