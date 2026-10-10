import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

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

const createRunTableColumns = (
  versionById: ReadonlyMap<string, bigint>,
): TableColumn<RunInfo>[] => [
  {
    id: "status",
    header: "Status",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
    cell: ({ row }) => (
      <PipelineHistoryRunStatus
        status={row.status}
        error={row.error}
        executionStatus={row.executionStatus}
      />
    ),
  },
  {
    id: "startedAt",
    header: "Started",
    cell: ({ row }) => {
      const { timestamp, isStarted } = getPipelineHistoryRunTimestamp(row);
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
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
    cell: ({ row }) => {
      const version = versionById.get(row.pipelineVersionId);
      return <Text size={TextSize.BODY_SM}>{version ? `Version ${version.toString()}` : "—"}</Text>;
    },
  },
  {
    id: "duration",
    header: "Duration",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
    cell: ({ row }) => (
      <PipelineHistoryRunDuration
        status={row.status}
        startedAt={row.startedAt}
        endedAt={row.endedAt}
      />
    ),
  },
  {
    id: "records",
    header: "Records",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.startedAt ? formatCount(row.records) : "—"}
      </Text>
    ),
  },
  {
    id: "volume",
    header: "Volume",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
    align: "right",
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.startedAt ? formatBytes(row.bytes) : "—"}
      </Text>
    ),
  },
];

const PipelineHistoryPage: FC = () => {
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
    <Box variant={BoxVariant.BASE} fillWidth height="100%" overflow="hidden">
      <Flex direction={FlexDirection.COLUMN} height="100%">
        <Box padding={16} fillWidth>
          <BaseHeader size={BaseHeaderSize.LARGE} title="History" />
        </Box>

        <Divider />

        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0} fillWidth>
          <InfiniteTable<RunInfo>
            columns={columns}
            data={runs}
            getRowId={(run) => run.id}
            emptyState={
              <EmptyLayout
                header="No runs yet"
                description="Run a pipeline to see its history here."
              />
            }
            expandedIds={runIds}
            onExpandedIdsChange={handleExpandedChange}
            renderExpandedRow={(row) => {
              return <PipelineHistoryRunInfo runId={row.id} />;
            }}
            isLoading={isFetchingNextPage}
            onEndReached={() => {
              if (hasNextPage) fetchNextPage();
            }}
          />
        </Flex>
      </Flex>
    </Box>
  );
};

export default PipelineHistoryPage;
