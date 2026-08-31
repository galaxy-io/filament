import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";

import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

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

const createRunTableColumns = (versionById: ReadonlyMap<string, bigint>): ColumnDef<RunInfo>[] => [
  {
    id: "status",
    header: "Status",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) => (
      <PipelineHistoryRunStatus status={row.original.status} error={row.original.error} />
    ),
  },
  {
    id: "startedAt",
    header: "Started",
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => {
      const { timestamp, isStarted } = getPipelineHistoryRunTimestamp(row.original);
      return (
        <Text
          size={TextSize.BODY_SM}
          variant={isStarted ? TextVariant.PRIMARY : TextVariant.TERTIARY}
          isEllipsis
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
    cellLoading: () => <TextShimmer width={32} height={14} />,
    cell: ({ row }) => {
      const version = versionById.get(row.original.pipelineVersionId);
      return <Text size={TextSize.BODY_SM}>{version ? `Version ${version.toString()}` : "—"}</Text>;
    },
  },
  {
    id: "duration",
    header: "Duration",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
    cellLoading: () => <TextShimmer width={160} height={14} />,
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
    cellLoading: () => <TextShimmer width={48} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.startedAt ? formatCount(row.original.records) : "—"}
      </Text>
    ),
  },
  {
    id: "volume",
    header: "Volume",
    size: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.startedAt ? formatBytes(row.original.bytes) : "—"}
      </Text>
    ),
  },
];

const PipelineHistoryPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const navigate = useNavigate();
  const { runId: runIds = [] } = useSearch({ from: "/pipelines/$id/history" });

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
      <Wrapper padding={"16px"} fillWidth>
        <BaseHeader size={BaseHeaderSize.LARGE} title="History" />
      </Wrapper>

      <HorizontalDivider />

      <RunTableWrapper>
        <InfiniteTable<RunInfo>
          columns={columns}
          data={runs}
          getRowId={(run) => run.id}
          contentWhenEmpty={
            <EmptyLayout header="No runs yet" message="Run a pipeline to see its history here." />
          }
          expandedRowIds={runIds}
          onExpandedChange={handleExpandedChange}
          onRowExpand={(row) => {
            return <PipelineHistoryRunInfo runId={row.original.id} />;
          }}
          hasNextPage={hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          fetchNextPage={fetchNextPage}
          fillWidth
          fillHeight
        />
      </RunTableWrapper>
    </PageWrapper>
  );
};

export default PipelineHistoryPage;
