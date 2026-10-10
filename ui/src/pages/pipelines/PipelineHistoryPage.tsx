import { type FC, useMemo } from "react";

import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunDuration from "@/components/runs/PipelineRunDuration";
import PipelineRunRecords from "@/components/runs/PipelineRunRecords";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";
import PipelineRunVolume from "@/components/runs/PipelineRunVolume";

import {
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/history/constants";
import PipelineHistoryRunInfo from "@/pages/pipelines/history/PipelineHistoryRunInfo";
import { getPipelineHistoryRunTimestamp } from "@/pages/pipelines/history/utils";

import {
  useFilamentSearchUpdate,
  usePipelineHistorySearch,
  usePipelineParams,
} from "@/module/hooks";
import type { PipelineHistorySearch } from "@/module/schemas";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import { useSuspenseListRunsInfiniteQuery } from "@/api/queries/runs";

import { formatTimestamp, formatVersion } from "@/utils/format";

const createRunTableColumns = (
  versionById: ReadonlyMap<string, bigint>,
): TableColumn<RunInfo>[] => [
  {
    id: "status",
    header: "Status",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS,
    cell: ({ row }) => (
      <PipelineRunStatus
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
      return <Text size={TextSize.BODY_SM}>{formatVersion(version)}</Text>;
    },
  },
  {
    id: "duration",
    header: "Duration",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
    cell: ({ row }) => <PipelineRunDuration run={row} />,
  },
  {
    id: "records",
    header: "Records",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS,
    cell: ({ row }) => <PipelineRunRecords run={row} />,
  },
  {
    id: "volume",
    header: "Volume",
    width: PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
    align: "right",
    cell: ({ row }) => <PipelineRunVolume run={row} />,
  },
];

const PipelineHistoryPage: FC = () => {
  const { id } = usePipelineParams();
  const updateSearch = useFilamentSearchUpdate<PipelineHistorySearch>();
  const { runId: runIds = [] } = usePipelineHistorySearch();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
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

  const runs = useMemo(() => data.pages.flatMap((page) => page.runs), [data.pages]);

  const handleExpandedChange = (expandedRowIds: string[]) => {
    void updateSearch(
      (prev) => ({
        ...prev,
        runId: expandedRowIds.length > 0 ? expandedRowIds : undefined,
      }),
      { replace: true },
    );
  };

  return (
    <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0} fillWidth>
      <InfiniteTable<RunInfo>
        columns={columns}
        data={runs}
        getRowId={(run) => run.id}
        emptyState={
          <EmptyLayout header="No runs yet" description="Run a pipeline to see its history here." />
        }
        expandedIds={runIds}
        onExpandedIdsChange={handleExpandedChange}
        renderExpandedRow={(row) =>
          row.status === RunStatus.SCHEDULED ? null : <PipelineHistoryRunInfo runId={row.id} />
        }
        isLoading={isFetchingNextPage}
        onEndReached={() => {
          if (hasNextPage) fetchNextPage();
        }}
      />
    </Flex>
  );
};

export default PipelineHistoryPage;
