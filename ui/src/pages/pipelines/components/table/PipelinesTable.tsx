import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  ColumnPin,
  type Row,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import {
  PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
  PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
  PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS,
  PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
} from "@/pages/pipelines/components/table/constants";
import PipelinesTableFlowCell from "@/pages/pipelines/components/table/PipelinesTableFlowCell";
import {
  PIPELINES_TABLE_COLUMN_ID_PIPELINE,
  type PipelinesTableSorting,
  type PipelinesTableSortingChange,
} from "@/pages/pipelines/components/table/utils";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import PipelinesTableColumnName from "./columns/PipelinesTableColumnName";
import PipelinesTableColumnRecentRuns from "./columns/PipelinesTableColumnRecentRuns";
import { formatCount, formatDuration, formatTimeAgo } from "@/utils/format";

const PipelinesTableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const PIPELINES_TABLE_COLUMNS: ColumnDef<Pipeline>[] = [
  {
    id: "flow",
    header: "Flow",
    size: PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
    pin: ColumnPin.LEFT,
    enableSorting: false,
    cellLoading: () => <TextShimmer width={120} height={18} />,
    cell: ({ row }) => <PipelinesTableFlowCell pipeline={row.original} />,
  },
  {
    id: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
    header: "Pipeline",
    minSize: PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
    accessorFn: (pipeline) => pipeline.name,
    enableSorting: true,
    sortDescFirst: false,
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => <PipelinesTableColumnName pipeline={row.original} />,
  },
  {
    id: "recentRuns",
    header: "Runs",
    size: PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS,
    enableSorting: false,
    cellLoading: () => <TextShimmer width={136} height={18} />,
    cell: ({ row }) => <PipelinesTableColumnRecentRuns pipeline={row.original} />,
  },
  {
    id: "lastRun",
    header: "Ran",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
    enableSorting: false,
    cellLoading: () => <TextShimmer width={64} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {row.original.lastRun ? formatTimeAgo(row.original.lastRun.requestedAt) : "—"}
      </Text>
    ),
  },
  {
    id: "status",
    header: "Status",
    size: PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
    enableSorting: false,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) =>
      row.original.lastRun ? (
        <PipelineHistoryRunStatus
          status={row.original.lastRun.status}
          error={row.original.lastRun.error}
        />
      ) : (
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          Never run
        </Text>
      ),
  },
  {
    id: "lastDuration",
    header: "Duration",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
    enableSorting: false,
    cellLoading: () => <TextShimmer width={48} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.lastRun
          ? formatDuration(row.original.lastRun.startedAt, row.original.lastRun.endedAt)
          : "—"}
      </Text>
    ),
  },
  {
    id: "lastVolume",
    header: "Records",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
    align: ColumnAlign.RIGHT,
    enableSorting: false,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.lastRun ? formatCount(row.original.lastRun.records) : "—"}
      </Text>
    ),
  },
];

interface PipelinesTableProps {
  pipelines: Pipeline[];
  sorting: PipelinesTableSorting;
  onSortingChange: PipelinesTableSortingChange;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  fetchNextPage?: () => void;
}

const PipelinesTable = ({
  pipelines,
  sorting,
  onSortingChange,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
}: PipelinesTableProps) => {
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
        hasNextPage={hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        fetchNextPage={fetchNextPage}
        enableSorting
        manualSorting
        sorting={sorting}
        onSortingChange={onSortingChange}
        fillWidth
        fillHeight
      />
    </PipelinesTableWrapper>
  );
};

export default PipelinesTable;
