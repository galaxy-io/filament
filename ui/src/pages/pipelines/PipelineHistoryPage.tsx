import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import InfiniteTable, { ColumnAlign } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";
import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";
import EmptyLayout from "@/layouts/EmptyLayout";

import {
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/history/constants";
import PipelineHistoryRunDuration from "@/pages/pipelines/history/PipelineHistoryRunDuration";
import PipelineHistoryRunInfo from "@/pages/pipelines/history/PipelineHistoryRunInfo";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";
import { getPipelineHistoryRunTimestamp } from "@/pages/pipelines/history/utils";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import { useSuspenseListRunsInfiniteQuery } from "@/api/queries/runs";

import { formatBytes, formatCount, formatTimestamp } from "@/utils/format";

const PageWrapper = styled.div`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${t.color.background.base};
`;

const RunTableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const createRunTableColumns = (
  versionById: ReadonlyMap<string, bigint>,
): TableColumn<RunInfo>[] => [
  {
    id: "status",
    header: "Status",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
    cellLoading: () => (
      <Box width={64}>
        <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
          height={18}
        />
      </Box>
    ),
    cell: ({ row }) => (
      <PipelineHistoryRunStatus
        status={row.original.status}
        error={row.original.error}
        executionStatus={row.original.executionStatus}
      />
    ),
  },
  {
    id: "startedAt",
    header: "Started",
    cellLoading: () => (
      <Box width={160}>
        <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
          height={14}
        />
      </Box>
    ),
    cell: ({ row }) => {
      const { timestamp, isStarted } = getPipelineHistoryRunTimestamp(row.original);
      return (
        <Text
          size={TextSize.BODY_SM}
          variant={isStarted ? TextVariant.PRIMARY : TextVariant.TERTIARY}
          lineClamp={1}
        >
          {formatTimestamp(timestamp)}
        </Text>
      );
    },
  },
  {
    id: "version",
    header: "Version",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
    cellLoading: () => (
      <Box width={32}>
        <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
          height={14}
        />
      </Box>
    ),
    cell: ({ row }) => {
      const version = versionById.get(row.original.pipelineVersionId);
      return <Text size={TextSize.BODY_SM}>{version ? `Version ${version.toString()}` : "—"}</Text>;
    },
  },
  {
    id: "duration",
    header: "Duration",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
    cellLoading: () => (
      <Box width={160}>
        <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
          height={14}
        />
      </Box>
    ),
    cell: ({ row }) => (
      <PipelineHistoryRunDuration
        status={row.original.status}
        startedAt={row.original.startedAt}
        endedAt={row.original.endedAt}
      />
    ),
  },
  {
    id: "records",
    header: "Records",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS,
    cellLoading: () => (
      <Box width={48}>
        <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
          height={14}
        />
      </Box>
    ),
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.original.startedAt ? formatCount(row.original.records) : "—"}
      </Text>
    ),
  },
  {
    id: "volume",
    header: "Volume",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
    // @dls-migrate infinitetable.ColumnAlign: Removed: use the 2.0 replacement where the value is passed around.
    align: ColumnAlign.RIGHT,
    cellLoading: () => (
      <Box width={52}>
        <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
          height={14}
        />
      </Box>
    ),
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.original.startedAt ? formatBytes(row.original.bytes) : "—"}
      </Text>
    ),
  },
];

const PipelineHistoryPage = () => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });
  const navigate = useNavigate();
  const { runId: runIds = [] } = useSearch({ from: "/_app/pipelines/$id/history" });

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeVersions: true }),
  });
  const columns = useMemo(() => {
    const versions = pipelineData.pipeline?.versions ?? [];
    const versionById = new Map(versions.map((version) => [version.id, version.version] as const));
    return createRunTableColumns(versionById);
  }, [pipelineData.pipeline]);

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } = useSuspenseListRunsInfiniteQuery(
    {
      input: { pipelineId: id },
    },
  );

  const runs = useMemo(() => {
    const seen = new Set<string>();
    return data.pages
      .flatMap((page) => page.runs)
      .filter((run) => {
        if (seen.has(run.id)) return false;
        seen.add(run.id);
        return true;
      });
  }, [data.pages]);

  const handleExpandedChange = (expandedRowIds: string[]) => {
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({
        ...prev,
        runId: expandedRowIds.length > 0 ? expandedRowIds : undefined,
      }),
    });
  };

  return (
    <PageWrapper>
      <Box padding={16} fillWidth>
        <BaseHeader size={BaseHeaderSize.LARGE} title="History" />
      </Box>

      <Divider />

      <RunTableWrapper>
        <Box height="100%">
          <InfiniteTable<RunInfo>
            columns={columns}
            data={runs}
            getRowId={(run) => run.id}
            emptyState={
              <EmptyLayout header="No runs yet" message="Run a pipeline to see its history here." />
            }
            expandedIds={runIds}
            onExpandedIdsChange={handleExpandedChange}
            renderExpandedRow={(row) => {
              return <PipelineHistoryRunInfo runId={row.original.id} />;
            }}
            isLoading={isFetchingNextPage}
            onEndReached={() => {
              if (hasNextPage) fetchNextPage();
            }}
          />
        </Box>
      </RunTableWrapper>
    </PageWrapper>
  );
};

export default PipelineHistoryPage;
