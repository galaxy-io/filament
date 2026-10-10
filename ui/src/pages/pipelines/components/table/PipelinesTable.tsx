import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import { useLocalStorage } from "@galaxy-io/dls/hooks/useLocalStorage";
import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn, TableColumnLayout, TableSort } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import { EMPTY_VALUE, formatNumber, formatRelativeTime } from "@galaxy-io/dls/utils/format";

import {
  type Pipeline,
  UpdatePipelineScheduleRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/pipelines/PipelineName";
import PipelineScheduleIndicator from "@/components/pipelines/PipelineScheduleIndicator";
import { formatPipelineName } from "@/components/pipelines/utils";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import PipelinesTableColumnFlow from "@/pages/pipelines/components/table/columns/PipelinesTableColumnFlow";
import PipelinesTableColumnRecentRuns from "@/pages/pipelines/components/table/columns/PipelinesTableColumnRecentRuns";
import {
  PIPELINES_TABLE_COLUMN_ID_PIPELINE,
  PIPELINES_TABLE_COLUMN_LAYOUT_STORAGE_KEY,
  PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
  PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_RECORDS,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
  PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS,
  PIPELINES_TABLE_COLUMN_WIDTH_STATUS,
} from "@/pages/pipelines/components/table/constants";
import PipelinesTableRowActions from "@/pages/pipelines/components/table/PipelinesTableRowActions";
import { usePipelineRun } from "@/pages/pipelines/hooks/usePipelineRun";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useUpdatePipelineScheduleMutation } from "@/api/queries/schedules";

import { getErrorMessage } from "@/utils/errors";
import { formatRunDuration } from "@/utils/runs";

const PIPELINES_TABLE_COLUMNS: TableColumn<Pipeline>[] = [
  {
    id: "flow",
    header: "Flow",
    width: PIPELINES_TABLE_COLUMN_WIDTH_FLOW,
    canSort: false,
    cell: ({ row }) => <PipelinesTableColumnFlow pipeline={row} />,
  },
  {
    id: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
    header: "Pipeline",
    minWidth: PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE,
    accessor: (pipeline) => pipeline.name,
    canSort: true,
    canHide: false,
    cell: ({ row }) => (
      <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
        <PipelineName pipelineId={row.id} pipeline={row} />
        <PipelineScheduleIndicator pipelineId={row.id} pipeline={row} />
      </Flex>
    ),
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
        {row.lastRun ? formatRelativeTime(row.lastRun.requestedAt) : EMPTY_VALUE}
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
        <PipelineRunStatus
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
        {row.lastRun ? formatRunDuration(row.lastRun.startedAt, row.lastRun.endedAt) : EMPTY_VALUE}
      </Text>
    ),
  },
  {
    id: "lastRecords",
    header: "Records",
    width: PIPELINES_TABLE_COLUMN_WIDTH_LAST_RECORDS,
    align: "right",
    canSort: false,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.lastRun ? formatNumber(row.lastRun.records) : EMPTY_VALUE}
      </Text>
    ),
  },
];

interface PipelinesTableProps {
  pipelines: Pipeline[];
  sorting: TableSort | null;
  onSortingChange: (sorting: TableSort | null) => void;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  fetchNextPage?: () => void;
}

const PipelinesTable: FC<PipelinesTableProps> = ({
  pipelines,
  sorting,
  onSortingChange,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
}) => {
  const navigate = useFilamentNavigate();
  const { toast } = useToast();

  const { startRun } = usePipelineRun();
  const { mutate: updateSchedule } = useUpdatePipelineScheduleMutation();

  const [columnLayout, setColumnLayout] = useLocalStorage<TableColumnLayout>(
    PIPELINES_TABLE_COLUMN_LAYOUT_STORAGE_KEY,
    {},
  );

  const handleRowClick = (row: Pipeline) => {
    void navigate({
      to: FilamentPath.PIPELINE,
      params: { id: row.id },
    });
  };

  const handleRun = (pipeline: Pipeline) => {
    startRun(pipeline);
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
    void navigate({ to: FilamentPath.PIPELINE_CANVAS, params: { id: pipeline.id } });
  };

  const handleSettings = (pipeline: Pipeline) => {
    void navigate({ to: FilamentPath.PIPELINE_SETTINGS, params: { id: pipeline.id } });
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
