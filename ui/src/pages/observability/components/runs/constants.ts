import { create } from "@bufbuild/protobuf";

import type { ChartSeriesStyles } from "@galaxy-io/dls/charts/types";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import { ListRunsRequestSchema, RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { SortingRequestSchema, SortOrder } from "@/gen/ingestion/v1/sorting_pb";

import {
  PIPELINE_RUN_EXECUTED_STATUSES,
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { createEnumSelectOptions, getEnumValues } from "@/utils/select";

export const OBSERVABILITY_RUNS_SERIES: ChartSeriesStyles<ObservabilityRunMetric> = {
  runs: { label: "Runs" },
};

export const OBSERVABILITY_RUN_STATUSES = getEnumValues(RunStatus);

export const OBSERVABILITY_RUNS_SCHEDULED_SERIES: ChartSeriesStyles<ObservabilityRunMetric> = {
  runs: {
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED],
    color: PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.SCHEDULED] ?? undefined,
  },
};

export const OBSERVABILITY_RUNS_VIEW_TO_LABEL_MAP: Record<ObservabilityRunsView, string> = {
  [ObservabilityRunsView.PAST]: "Past",
  [ObservabilityRunsView.UPCOMING]: "Upcoming",
};

export const OBSERVABILITY_RUNS_EMPTY_STATE_TEXT_MAP: Record<ObservabilityRunsView, string> = {
  [ObservabilityRunsView.PAST]: "No runs in the selected timeframe",
  [ObservabilityRunsView.UPCOMING]: "No upcoming runs",
};

export const OBSERVABILITY_RUN_STATUS_OPTIONS = createEnumSelectOptions(
  PIPELINE_RUN_EXECUTED_STATUSES,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
);

export const OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION: SelectOption = {
  id: String(RunStatus.SCHEDULED),
  label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED],
};

export const OBSERVABILITY_RUNS_ALL_STATUSES_LABEL = "All statuses";

export const OBSERVABILITY_RUNS_DEFAULT_STATUSES: RunStatus[] = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

export const OBSERVABILITY_RUNS_SCHEDULED_INPUT = create(ListRunsRequestSchema, {
  status: [RunStatus.SCHEDULED],
  sorting: create(SortingRequestSchema, { sortOrder: SortOrder.ASC }),
});

export const OBSERVABILITY_RUNS_TABLE_COLUMN_ID_STARTED_AT = "startedAt";

export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS = 110;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE = 240;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT = 160;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY = 100;
export const OBSERVABILITY_RUNS_TABLE_HEIGHT = 450;

export const OBSERVABILITY_RUNS_CHART_HEIGHT = 250;
export const OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH = 160;
export const OBSERVABILITY_RUNS_CHART_MIN_SEGMENT_LENGTH = 4;
