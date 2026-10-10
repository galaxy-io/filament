import { type FC, useMemo } from "react";

import Box from "@galaxy-io/dls/layout/Box";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS } from "@/pages/observability/components/runs/columns/constants";
import {
  OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP,
  OBSERVABILITY_RUNS_SCHEDULED_INPUT,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
  OBSERVABILITY_RUNS_TABLE_HEIGHT,
} from "@/pages/observability/components/runs/constants";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useListRunsQuery } from "@/api/queries/runs";

import { formatTimestamp } from "@/utils/format";

const OBSERVABILITY_RUNS_UPCOMING_TABLE_COLUMNS: TableColumn<RunInfo>[] = [
  ...OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS,
  {
    id: "scheduledAt",
    header: "Scheduled",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT,
    align: "right",
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {formatTimestamp(row.scheduledAt)}
      </Text>
    ),
  },
];

const ObservabilityRunsUpcomingTable: FC = () => {
  const navigate = useFilamentNavigate();

  const { data, isLoading } = useListRunsQuery({ input: OBSERVABILITY_RUNS_SCHEDULED_INPUT });

  const runs = useMemo(
    () => [...(data?.runs ?? [])].sort((a, b) => Number(a.scheduledAt - b.scheduledAt)),
    [data],
  );

  const handleRowClick = (row: RunInfo) => {
    navigate({
      to: FilamentPath.PIPELINE_HISTORY,
      params: { id: row.pipelineId },
      search: { runId: [row.id] },
    });
  };

  return (
    <Box height={OBSERVABILITY_RUNS_TABLE_HEIGHT}>
      <InfiniteTable<RunInfo>
        columns={OBSERVABILITY_RUNS_UPCOMING_TABLE_COLUMNS}
        data={runs}
        getRowId={(run) => run.id}
        onRowClick={handleRowClick}
        isLoading={isLoading}
        emptyState={
          <EmptyLayout
            size={EmptyLayoutSize.SMALL}
            header={OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP[ObservabilityRunsView.UPCOMING]}
          />
        }
      />
    </Box>
  );
};

export default ObservabilityRunsUpcomingTable;
