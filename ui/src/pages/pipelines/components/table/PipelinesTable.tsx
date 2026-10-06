import { create } from "@bufbuild/protobuf";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import { useLocalStorage } from "@galaxy-io/dls/hooks/useLocalStorage";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn, TableColumnLayout } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  type Pipeline,
  UpdatePipelineScheduleRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";
import { RunPipelineRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import EmptyLayout from "@/layouts/EmptyLayout";

import {
  PIPELINES_TABLE_COLUMN_LAYOUT_STORAGE_KEY,
  PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
  PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
  PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS,
  PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
} from "@/pages/pipelines/components/table/constants";
import PipelinesTableFlowCell from "@/pages/pipelines/components/table/PipelinesTableFlowCell";
import PipelinesTableRowActions from "@/pages/pipelines/components/table/PipelinesTableRowActions";
import {
  PIPELINES_TABLE_COLUMN_ID_PIPELINE,
  type PipelinesTableSorting,
  type PipelinesTableSortingChange,
} from "@/pages/pipelines/components/table/utils";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useRunPipelineMutation } from "@/api/queries/runs";
import { useUpdatePipelineScheduleMutation } from "@/api/queries/schedules";

import PipelinesTableColumnRecentRuns from "./columns/PipelinesTableColumnRecentRuns";
import { getErrorMessage } from "@/utils/errors";
import { formatCount, formatDuration, formatTimeAgo } from "@/utils/format";

const PIPELINES_TABLE_COLUMNS: TableColumn<Pipeline>[] = [
  {
    id: "flow",
    header: "Flow",
    width: PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
    canSort: false,
    cell: ({ row }) => <PipelinesTableFlowCell pipeline={row} />,
  },
  {
    id: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
    header: "Pipeline",
    minWidth: PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
    accessor: (pipeline) => pipeline.name,
    canSort: true,
    canHide: false,
    cell: ({ row }) => <PipelineName pipelineId={row.id} pipeline={row} />,
  },
  {
    id: "recentRuns",
    header: "Runs",
    width: PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS,
    canSort: false,
    cell: ({ row }) => <PipelinesTableColumnRecentRuns pipeline={row} />,
  },
  {
    id: "lastRun",
    header: "Ran",
    width: PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
    canSort: false,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {row.lastRun ? formatTimeAgo(row.lastRun.requestedAt) : "—"}
      </Text>
    ),
  },
  {
    id: "status",
    header: "Status",
    width: PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
    canSort: false,
    cell: ({ row }) =>
      row.lastRun ? (
        <PipelineHistoryRunStatus
          status={row.lastRun.status}
          error={row.lastRun.error}
          executionStatus={row.lastRun.executionStatus}
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
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.lastRun ? formatDuration(row.lastRun.startedAt, row.lastRun.endedAt) : "—"}
      </Text>
    ),
  },
  {
    id: "lastVolume",
    header: "Records",
    width: PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
    align: "right",
    canSort: false,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.lastRun ? formatCount(row.lastRun.records) : "—"}
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
  const { toast } = useToast();

  const { mutate: runPipeline } = useRunPipelineMutation();
  const { mutate: updateSchedule } = useUpdatePipelineScheduleMutation();

  const [columnLayout, setColumnLayout] = useLocalStorage<TableColumnLayout>(
    PIPELINES_TABLE_COLUMN_LAYOUT_STORAGE_KEY,
    {},
  );

  const handleRowClick = (row: Pipeline) => {
    navigate({
      to: "/pipelines/$id",
      params: { id: row.id },
    });
  };

  const handleRun = (pipeline: Pipeline) => {
    runPipeline(
      create(RunPipelineRequestSchema, {
        pipelineId: pipeline.id,
        options: { executionMode: pipeline.executionMode },
      }),
      {
        onSuccess: () => {
          toast({
            header: "Run started",
            description: `${formatPipelineName(pipeline)} is now running.`,
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          toast({
            header: "Run failed",
            description: getErrorMessage(error, "Failed to run pipeline"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const handleScheduleToggle = (pipeline: Pipeline) => {
    const config = pipeline.schedule?.config;
    if (!config) return;
    const isEnabled = !config.isEnabled;
    updateSchedule(
      create(UpdatePipelineScheduleRequestSchema, {
        pipelineId: pipeline.id,
        schedule: { ...config, isEnabled },
      }),
      {
        onSuccess: () => {
          toast({
            header: isEnabled ? "Schedule resumed" : "Schedule paused",
            description: isEnabled
              ? `${formatPipelineName(pipeline)} will run on its schedule.`
              : `${formatPipelineName(pipeline)} will only run on demand.`,
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          toast({
            header: "Schedule update failed",
            description: getErrorMessage(error, "Failed to update schedule"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const handleEdit = (pipeline: Pipeline) => {
    void navigate({ to: "/pipelines/$id/canvas", params: { id: pipeline.id } });
  };

  const handleSettings = (pipeline: Pipeline) => {
    void navigate({ to: "/pipelines/$id/settings", params: { id: pipeline.id } });
  };

  return (
    <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0} fillWidth>
      <InfiniteTable<Pipeline>
        columns={PIPELINES_TABLE_COLUMNS}
        data={pipelines}
        getRowId={(pipeline) => pipeline.id}
        emptyState={
          <EmptyLayout icon={MagnifyingGlassIcon} header="No pipelines match your search" />
        }
        onRowClick={handleRowClick}
        isLoading={isFetchingNextPage}
        onEndReached={() => {
          if (hasNextPage) fetchNextPage?.();
        }}
        sort={sorting}
        onSortChange={onSortingChange}
        rowActions={(row) => (
          <PipelinesTableRowActions
            pipeline={row}
            onRun={handleRun}
            onScheduleToggle={handleScheduleToggle}
            onEdit={handleEdit}
            onSettings={handleSettings}
          />
        )}
        canCustomizeColumns
        columnLayout={columnLayout}
        onColumnLayoutChange={setColumnLayout}
        ariaLabel="Pipelines"
      />
    </Flex>
  );
};

export default PipelinesTable;
