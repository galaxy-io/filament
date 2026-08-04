import type { ChartSeriesStyles } from "@galaxy-io/dls/charts/types";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import type { PinnedOptions } from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";

export const OBSERVABILITY_RUNS_SERIES: ChartSeriesStyles<ObservabilityRunMetric> = {
  runs: { label: "Runs" },
};

export const OBSERVABILITY_RUN_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
  RunStatus.REQUESTED,
  RunStatus.CANCELED,
  RunStatus.PAUSED,
  RunStatus.PARTIAL,
];

export const OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP: Partial<Record<RunStatus, ChartPalette>> = {
  [RunStatus.COMPLETED]: ChartPalette.GREEN,
  [RunStatus.FAILED]: ChartPalette.RED,
  [RunStatus.RUNNING]: ChartPalette.BLUE,
  [RunStatus.REQUESTED]: ChartPalette.YELLOW,
  [RunStatus.CANCELED]: ChartPalette.PURPLE,
  [RunStatus.PAUSED]: ChartPalette.ORANGE,
  [RunStatus.PARTIAL]: ChartPalette.TEAL,
};

export const OBSERVABILITY_RUN_STATUS_OPTIONS: SelectInputOption[] = OBSERVABILITY_RUN_STATUSES.map(
  (status) => ({
    id: String(status),
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
    value: status,
  }),
);

export const OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION: PinnedOptions = {
  id: "all-statuses",
  label: "All statuses",
  optionIds: OBSERVABILITY_RUN_STATUS_OPTIONS.map((option) => option.id),
};

export const OBSERVABILITY_RUNS_DEFAULT_STATUS_OPTIONS: SelectInputOption[] =
  OBSERVABILITY_RUN_STATUS_OPTIONS.filter(
    (option) => option.value === RunStatus.COMPLETED || option.value === RunStatus.FAILED,
  );

export const OBSERVABILITY_RUNS_TABLE_LIMIT = 50;
