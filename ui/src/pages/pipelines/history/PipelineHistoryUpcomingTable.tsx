import { useMemo } from "react";

import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import {
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
  PIPELINE_HISTORY_UPCOMING_TABLE_MAX_HEIGHT,
} from "@/pages/pipelines/history/constants";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { formatTimestamp, formatTimeUntil } from "@/utils/format";

interface PipelineHistoryUpcomingTableProps {
  runs: RunInfo[];
  versionById: ReadonlyMap<string, bigint>;
}

const PipelineHistoryUpcomingTable = ({ runs, versionById }: PipelineHistoryUpcomingTableProps) => {
  const columns = useMemo<ColumnDef<RunInfo>[]>(
    () => [
      {
        id: "status",
        header: "Status",
        size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
        cellLoading: () => <TextShimmer width={64} height={18} />,
        cell: ({ row }) => <PipelineHistoryRunStatus status={row.original.status} />,
      },
      {
        id: "scheduledAt",
        header: "Scheduled for",
        cellLoading: () => <TextShimmer width={160} height={14} />,
        cell: ({ row }) => (
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <Text size={TextSize.BODY_SM} isEllipsis>
              {formatTimestamp(row.original.scheduledAt)}
            </Text>
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
              {formatTimeUntil(row.original.scheduledAt)}
            </Text>
          </FlexWrapper>
        ),
      },
      {
        id: "version",
        header: "Version",
        size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
        align: ColumnAlign.RIGHT,
        cellLoading: () => <TextShimmer width={32} height={14} />,
        cell: ({ row }) => {
          const version = versionById.get(row.original.pipelineVersionId);
          return (
            <Text size={TextSize.BODY_SM}>{version ? `Version ${version.toString()}` : "—"}</Text>
          );
        },
      },
    ],
    [versionById],
  );

  return (
    <>
      <BaseToolbar
        leadingActions={[
          <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
            Upcoming
          </Text>,
        ]}
      />
      <HorizontalDivider />
      <InfiniteTable<RunInfo>
        columns={columns}
        data={runs}
        getRowId={(run) => run.id}
        maxHeight={PIPELINE_HISTORY_UPCOMING_TABLE_MAX_HEIGHT}
        fillWidth
        noLastRowPadding
        noLastRowBorder
      />
      <HorizontalDivider />
    </>
  );
};

export default PipelineHistoryUpcomingTable;
