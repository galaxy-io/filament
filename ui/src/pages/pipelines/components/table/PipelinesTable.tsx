import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box from "@galaxy-io/dls/layout/Box";
// @dls-migrate infinitetable.Row: Removed: TanStack types are not exposed; use `TableColumn`, `TableColumnLayout`, `TableCellContext`, `TableSort`.
import InfiniteTable, { type Row } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily, Side } from "@galaxy-io/dls/theme/enums";

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

const PIPELINES_TABLE_COLUMNS: TableColumn<Pipeline>[] = [
  {
    id: "flow",
    header: "Flow",
    width: PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
    pin: Side.LEFT,
    canSort: false,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => <PipelinesTableFlowCell pipeline={row.original} />,
  },
  {
    id: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
    header: "Pipeline",
    minWidth: PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
    accessor: (pipeline) => pipeline.name,
    canSort: true,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => <PipelinesTableColumnName pipeline={row.original} />,
  },
  {
    id: "recentRuns",
    header: "Runs",
    width: PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS,
    canSort: false,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => <PipelinesTableColumnRecentRuns pipeline={row.original} />,
  },
  {
    id: "lastRun",
    header: "Ran",
    width: PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
    canSort: false,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {row.original.lastRun ? formatTimeAgo(row.original.lastRun.requestedAt) : "—"}
      </Text>
    ),
  },
  {
    id: "status",
    header: "Status",
    width: PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
    canSort: false,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) =>
      row.original.lastRun ? (
        <PipelineHistoryRunStatus
          status={row.original.lastRun.status}
          error={row.original.lastRun.error}
          executionStatus={row.original.lastRun.executionStatus}
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
    width: PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
    canSort: false,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.original.lastRun
          ? formatDuration(row.original.lastRun.startedAt, row.original.lastRun.endedAt)
          : "—"}
      </Text>
    ),
  },
  {
    id: "lastVolume",
    header: "Records",
    width: PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
    align: "right",
    canSort: false,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
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
      <Box height="100%">
        <InfiniteTable<Pipeline>
          columns={PIPELINES_TABLE_COLUMNS}
          data={pipelines}
          getRowId={(pipeline) => pipeline.id}
          emptyState={
            <EmptyLayout
              icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
              message="No pipelines match your search"
            />
          }
          /* @dls-migrate infinitetable.onRowClick: `row` is now the data object: `row.original` → `row`. */ onRowClick={
            handleRowClick
          }
          isLoading={isFetchingNextPage}
          onEndReached={() => {
            if (hasNextPage) fetchNextPage();
          }}
          /* @dls-migrate infinitetable.enableSorting: Sorting is per column (`canSort`) with a `TableSort` value. */ enableSorting
          /* @dls-migrate infinitetable.sorting: Sorting is per column (`canSort`) with a `TableSort` value. */ sorting={
            sorting
          }
          /* @dls-migrate infinitetable.onSortingChange: Sorting is per column (`canSort`) with a `TableSort` value. */ onSortingChange={
            onSortingChange
          }
        />
      </Box>
    </PipelinesTableWrapper>
  );
};

export default PipelinesTable;
