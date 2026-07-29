import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useParams } from "@tanstack/react-router";

import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ListRunsRequestSchema, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";
import EmptyLayout from "@/layouts/EmptyLayout";

import {
  PIPELINE_RUN_HISTORY_LIMIT,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_DURATION,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_RECORDS,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_STARTED_AT,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_STATUS,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_VERSION,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/history/constants";
import PipelineRunInfo from "@/pages/pipelines/history/PipelineRunInfo";
import PipelineRunStatus from "@/pages/pipelines/history/PipelineRunStatus";

import { useSuspenseListRunsQuery } from "@/api/queries/runs";

import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/utils/format";

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
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_STATUS,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) => (
      <PipelineRunStatus status={row.original.status} error={row.original.error} />
    ),
  },
  {
    id: "startedAt",
    header: "Started",
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_STARTED_AT,
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
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_DURATION,
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
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_VERSION,
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
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_RECORDS,
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
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_VOLUME,
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

  const { data } = useSuspenseListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      limit: PIPELINE_RUN_HISTORY_LIMIT,
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
          data={data.runs}
          getRowId={(run) => run.runId}
          contentWhenEmpty={
            <EmptyLayout header="No runs yet" message="Run a pipeline to see its history here." />
          }
          onRowExpand={(row) => {
            return <PipelineRunInfo runId={row.original.runId} />;
          }}
          fillWidth
          fillHeight
        />
      </RunTableWrapper>
    </PageWrapper>
  );
};

export default PipelineHistoryPage;
