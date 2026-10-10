import type { TableColumn } from "@galaxy-io/dls/table/types";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/pipelines/PipelineName";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import ObservabilityRunsTableColumnFlow from "@/pages/observability/components/runs/columns/ObservabilityRunsTableColumnFlow";
import {
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE,
  OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
} from "@/pages/observability/components/runs/constants";

export const OBSERVABILITY_RUNS_TABLE_BASE_COLUMNS: TableColumn<RunInfo>[] = [
  {
    id: "status",
    header: "Status",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS,
    canSort: false,
    cell: ({ row }) => (
      <PipelineRunStatus
        status={row.status}
        error={row.error}
        executionStatus={row.executionStatus}
      />
    ),
  },
  {
    id: "flow",
    header: "Flow",
    width: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW,
    canSort: false,
    cell: ({ row }) => <ObservabilityRunsTableColumnFlow runInfo={row} />,
  },
  {
    id: "pipeline",
    header: "Pipeline",
    minWidth: OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE,
    canSort: false,
    cell: ({ row }) => <PipelineName pipelineId={row.pipelineId} />,
  },
];
