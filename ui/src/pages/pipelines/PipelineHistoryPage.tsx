import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";

import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";
import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import RunStatusCell from "@/pages/pipelines/components/RunStatusCell";
import {
  RUN_HISTORY_LIMIT,
  RUN_HISTORY_LOADING_ROW_COUNT,
  RUN_TABLE_COLUMN_WIDTH_DURATION,
  RUN_TABLE_COLUMN_WIDTH_RECORDS,
  RUN_TABLE_COLUMN_WIDTH_STARTED_AT,
  RUN_TABLE_COLUMN_WIDTH_STATUS,
  RUN_TABLE_COLUMN_WIDTH_VERSION,
  RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/constants";
import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/pages/pipelines/utils";

import { useListRunsQuery } from "@/api/queries/runs";

import { ListRunsRequestSchema, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import PipelineHistoryRunInfo from "./canvas/history/PipelineHistoryRunInfo";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const RunTableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const RUN_TABLE_COLUMNS: ColumnDef<RunInfo>[] = [
  {
    id: "status",
    header: "Status",
    size: RUN_TABLE_COLUMN_WIDTH_STATUS,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) => <RunStatusCell status={row.original.status} error={row.original.error} />,
  },
  {
    id: "startedAt",
    header: "Started",
    size: RUN_TABLE_COLUMN_WIDTH_STARTED_AT,
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {formatTimestamp(row.original.startedAt)}
      </Text>
    ),
  },
  {
    id: "run",
    header: "Run",
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isMonospace isEllipsis>
        {row.original.runId}
      </Text>
    ),
  },
  {
    id: "duration",
    header: "Duration",
    size: RUN_TABLE_COLUMN_WIDTH_DURATION,
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {formatDuration(row.original.startedAt, row.original.endedAt)}
      </Text>
    ),
  },
  {
    id: "version",
    header: "Version",
    size: RUN_TABLE_COLUMN_WIDTH_VERSION,
    cellLoading: () => <TextShimmer width={32} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM}>
        {row.original.pipelineVersionId ? `Version ${row.original.pipelineVersionId}` : "—"}
      </Text>
    ),
  },
  {
    id: "records",
    header: "Records",
    size: RUN_TABLE_COLUMN_WIDTH_RECORDS,
    align: ColumnAlign.CENTER,
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
    size: RUN_TABLE_COLUMN_WIDTH_VOLUME,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatBytes(row.original.bytes)}
      </Text>
    ),
  },
];

const PipelineHistoryPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data, isLoading, isError } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      limit: RUN_HISTORY_LIMIT,
    }),
  });

  return (
    <PageWrapper>
      <Wrapper padding={"16px"} fillWidth>
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title="History"
          description="Review recent pipeline runs."
        />
      </Wrapper>

      <HorizontalDivider />

      <RunTableWrapper>
        <InfiniteTable<RunInfo>
          columns={RUN_TABLE_COLUMNS}
          data={data?.runs ?? []}
          getRowId={(run) => run.runId}
          isLoading={isLoading}
          loadingRowCount={RUN_HISTORY_LOADING_ROW_COUNT}
          isError={isError}
          contentWhenError={
            <ErrorLayout
              icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
              message="Failed to load runs. Please try again."
            />
          }
          contentWhenEmpty={
            <EmptyLayout header="No runs yet" message="Run a pipeline to see its history here." />
          }
          onRowExpand={(row) => {
            return <PipelineHistoryRunInfo runId={row.original.runId} />;
          }}
          fillWidth
          fillHeight
        />
      </RunTableWrapper>
    </PageWrapper>
  );
};

export default PipelineHistoryPage;
