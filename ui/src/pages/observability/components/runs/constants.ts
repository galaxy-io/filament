import { create } from "@bufbuild/protobuf";

import type { ChartSeriesStyles } from "@galaxy-io/dls/charts/types";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import type { PinnedOptions } from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { ListRunsRequestSchema, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import { ObservabilityRunsView } from "@/pages/observability/types";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";

export const OBSERVABILITY_RUNS_SERIES: ChartSeriesStyles<ObservabilityRunMetric> = {
  runs: { label: "Runs" },
};

export const OBSERVABILITY_RUN_STATUSES = Object.values(RunStatus).filter(
  (status): status is RunStatus => typeof status === "number" && status !== RunStatus.UNSPECIFIED,
);

export const OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP: Record<RunStatus, ChartPalette | undefined> = {
  [RunStatus.UNSPECIFIED]: undefined,
  [RunStatus.COMPLETED]: ChartPalette.GREEN,
  [RunStatus.FAILED]: ChartPalette.RED,
  [RunStatus.RUNNING]: ChartPalette.BLUE,
  [RunStatus.REQUESTED]: ChartPalette.LIME,
  [RunStatus.SCHEDULED]: ChartPalette.YELLOW,
  [RunStatus.CANCELED]: ChartPalette.PURPLE,
  [RunStatus.PAUSED]: ChartPalette.ORANGE,
  [RunStatus.PARTIAL]: ChartPalette.PINK,
};

export const OBSERVABILITY_RUNS_SCHEDULED_SERIES: ChartSeriesStyles<ObservabilityRunMetric> = {
  runs: {
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED],
    color: OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP[RunStatus.SCHEDULED],
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

export const OBSERVABILITY_RUN_STATUS_OPTIONS: SelectInputOption[] =
  OBSERVABILITY_RUN_STATUSES.filter((status) => status !== RunStatus.SCHEDULED).map((status) => ({
    id: String(status),
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
    value: status,
  }));

export const OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION: SelectInputOption = {
  id: String(RunStatus.SCHEDULED),
  label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED],
  value: RunStatus.SCHEDULED,
};

export const OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION: PinnedOptions = {
  id: "all-statuses",
  label: "All statuses",
  optionIds: OBSERVABILITY_RUN_STATUS_OPTIONS.map((option) => option.id),
};

export const OBSERVABILITY_RUNS_DEFAULT_STATUSES: RunStatus[] = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

export const OBSERVABILITY_RUNS_TABLE_LIMIT = 50;

export const OBSERVABILITY_RUNS_SCHEDULED_INPUT = create(ListRunsRequestSchema, {
  status: [RunStatus.SCHEDULED],
  pagination: create(PaginationRequestSchema, { total: OBSERVABILITY_RUNS_TABLE_LIMIT }),
});

export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STATUS = 110;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_PIPELINE = 240;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_FLOW = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_STARTED_AT = 160;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_DURATION = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_RECORDS = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_VOLUME = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_CPU = 100;
export const OBSERVABILITY_RUNS_TABLE_COLUMN_WIDTH_MEMORY = 100;
export const OBSERVABILITY_RUNS_TABLE_EMPTY_STATE_HEIGHT = 360;
export const OBSERVABILITY_RUNS_TABLE_HEIGHT = 450;

export const OBSERVABILITY_RUNS_CHART_HEIGHT = 250;
export const OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH = 160;
