import { type FC, useMemo } from "react";

import Box from "@galaxy-io/dls/layout/Box";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { EMPTY_VALUE, formatBytes, formatDuration } from "@galaxy-io/dls/utils/format";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunDuration from "@/components/runs/PipelineRunDuration";
import PipelineRunRecords from "@/components/runs/PipelineRunRecords";
import PipelineRunVolume from "@/components/runs/PipelineRunVolume";

import { OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS } from "@/pages/observability/components/runs/columns/constants";
import {
  OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_TIMESTAMP,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME,
  OBSERVABILITY_RUNS_TABLE_HEIGHT,
} from "@/pages/observability/components/runs/constants";
import { createRunsWindowInput } from "@/pages/observability/components/runs/utils";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { useFilamentNavigate, useObservabilitySearch } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useListRunsInfiniteQuery } from "@/api/queries/runs";

import { formatTimestamp } from "@/utils/format";

const OBSERVABILITY_RUNS_PAST_TABLE_COLUMNS: TableColumn<RunInfo>[] = [
  ...OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS,
  {
    id: "startedAt",
    header: "Started",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_TIMESTAMP,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {formatTimestamp(row.startedAt)}
      </Text>
    ),
  },
  {
    id: "duration",
    header: "Duration",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION,
    align: "right",
    cell: ({ row }) => <PipelineRunDuration run={row} />,
  },
  {
    id: "records",
    header: "Records",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS,
    align: "right",
    cell: ({ row }) => <PipelineRunRecords run={row} />,
  },
  {
    id: "volume",
    header: "Volume",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME,
    align: "right",
    cell: ({ row }) => <PipelineRunVolume run={row} />,
  },
  {
    id: "cpu",
    header: "CPU",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU,
    align: "right",
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.cpuSeconds ? formatDuration(row.cpuSeconds * 1_000) : EMPTY_VALUE}
      </Text>
    ),
  },
  {
    id: "memory",
    header: "Memory",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY,
    align: "right",
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
        {row.memoryPeakBytes ? formatBytes(row.memoryPeakBytes) : EMPTY_VALUE}
      </Text>
    ),
  },
];

const ObservabilityRunsPastTable: FC = () => {
  const navigate = useFilamentNavigate();
  const { timeframe, statuses, runsBucket, runsStatus } = useObservabilitySearch();

  const bucketStartMs = useObservabilityBucketStart(timeframe);

  const windowedStatuses = useMemo(
    () => statuses.filter((status) => status !== RunStatus.SCHEDULED),
    [statuses],
  );

  const input = useMemo(
    () => ({
      status: runsStatus === undefined ? windowedStatuses : [runsStatus],
      ...createRunsWindowInput(timeframe, bucketStartMs, runsBucket),
    }),
    [timeframe, bucketStartMs, windowedStatuses, runsBucket, runsStatus],
  );

  const { data, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useListRunsInfiniteQuery({
      input,
      options: { enabled: windowedStatuses.length > 0 },
    });

  const runs = windowedStatuses.length ? (data?.pages.flatMap((page) => page.runs) ?? []) : [];

  const handleRowClick = (row: RunInfo) => {
    void navigate({
      to: FilamentPath.PIPELINE_HISTORY,
      params: { id: row.pipelineId },
      search: { runId: [row.id] },
    });
  };

  return (
    <Box height={OBSERVABILITY_RUNS_TABLE_HEIGHT}>
      <InfiniteTable<RunInfo>
        columns={OBSERVABILITY_RUNS_PAST_TABLE_COLUMNS}
        data={runs}
        getRowId={(run) => run.id}
        onRowClick={handleRowClick}
        isLoading={isLoading || isFetchingNextPage}
        onEndReached={() => {
          if (hasNextPage) fetchNextPage();
        }}
        ariaLabel="Recent runs"
        emptyState={
          <EmptyLayout
            size={EmptyLayoutSize.SMALL}
            header={OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[ObservabilityRunsView.PAST]}
          />
        }
      />
    </Box>
  );
};

export default ObservabilityRunsPastTable;
