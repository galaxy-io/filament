import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable, { InfiniteTableSize } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { formatBytes, formatNumber } from "@galaxy-io/dls/utils/format";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import {
  GetRunRequestSchema,
  type RunInfo,
  type RunResourceState,
  RunStatus,
} from "@/gen/ingestion/v1/runs_pb";

import PipelineHistoryRunInfoConnectionColumn from "@/pages/pipelines/history/components/PipelineHistoryRunInfoConnectionColumn";
import {
  PIPELINE_HISTORY_RUN_INFO_LOADING_WIDTH,
  PIPELINE_HISTORY_RUN_INFO_TABLE_INSET,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
  PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/history/constants";
import PipelineHistoryRunContinuousSummary from "@/pages/pipelines/history/PipelineHistoryRunContinuousSummary";

import { useGetRunQuery } from "@/api/queries/runs";

import { formatTimestamp } from "@/utils/format";

interface PipelineHistoryRunInfoProps {
  runId: RunInfo["id"];
}

const PipelineHistoryRunInfo: FC<PipelineHistoryRunInfoProps> = ({ runId }) => {
  const { data, isLoading, isError } = useGetRunQuery({
    input: create(GetRunRequestSchema, { runId }),
  });
  const sourceConnectionId = data?.snapshot?.run?.sourceConnectionId ?? "";
  const sinkConnectionId = data?.snapshot?.run?.sinkConnectionId ?? "";

  const columns = useMemo<TableColumn<RunResourceState>[]>(
    () => [
      {
        id: "resource",
        header: "Resource",
        isRowHeader: true,
        cell: ({ row }) => (
          <PipelineHistoryRunInfoConnectionColumn
            connectionId={sourceConnectionId}
            resourceName={row.resourceName}
          />
        ),
      },
      {
        id: "sink",
        header: "Sink",
        width:
          PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION +
          PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION,
        cell: () => <PipelineHistoryRunInfoConnectionColumn connectionId={sinkConnectionId} />,
      },
      {
        id: "records",
        header: "Records",
        width:
          PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS - PIPELINE_HISTORY_RUN_INFO_TABLE_INSET,
        align: "right",
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {formatNumber(row.records)}
          </Text>
        ),
      },
      {
        id: "volume",
        header: "Volume",
        width:
          PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME - PIPELINE_HISTORY_RUN_INFO_TABLE_INSET,
        align: "right",
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {formatBytes(row.bytes)}
          </Text>
        ),
      },
    ],
    [sourceConnectionId, sinkConnectionId],
  );

  const resources = data?.snapshot?.resources ?? [];
  const isEmpty = resources.length === 0;

  const table = (
    <InfiniteTable<RunResourceState>
      ariaLabel="Run resources"
      columns={columns}
      data={resources}
      getRowId={(resource) => resource.resourceName}
      size={InfiniteTableSize.SMALL}
      inset={PIPELINE_HISTORY_RUN_INFO_TABLE_INSET}
    />
  );

  const renderBody = () => {
    if (isError) {
      return (
        <Flex alignItems={AlignItems.START} padding={16} fillWidth>
          <Text variant={TextVariant.ERROR}>Failed to load run details.</Text>
        </Flex>
      );
    }

    if (isLoading) {
      return (
        <Flex alignItems={AlignItems.CENTER} padding={16} fillWidth>
          <Box width={PIPELINE_HISTORY_RUN_INFO_LOADING_WIDTH}>
            <Skeleton />
          </Box>
        </Flex>
      );
    }

    if (data?.snapshot?.run?.executionMode === ExecutionMode.CONTINUOUS) {
      return (
        <>
          <PipelineHistoryRunContinuousSummary run={data.snapshot.run} />
          {!isEmpty && (
            <>
              <Divider />
              {table}
            </>
          )}
        </>
      );
    }

    if (data?.snapshot?.run?.status === RunStatus.CANCELED && !data.snapshot.run.startedAt) {
      const canceledAt = data.snapshot.run.endedAt;
      return (
        <Flex alignItems={AlignItems.START} padding={16} fillWidth>
          <Text variant={TextVariant.TERTIARY}>
            {canceledAt
              ? `The run was cancelled at ${formatTimestamp(canceledAt)}, before it started.`
              : "The run was cancelled before it started."}
          </Text>
        </Flex>
      );
    }

    if (isEmpty) {
      return (
        <Flex alignItems={AlignItems.START} padding={16} fillWidth>
          <Text variant={TextVariant.TERTIARY}>The run did not record any resource activity.</Text>
        </Flex>
      );
    }

    return table;
  };

  return (
    <Box variant={BoxVariant.PRIMARY} fillWidth height="100%">
      {renderBody()}
    </Box>
  );
};

export default PipelineHistoryRunInfo;
