import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/types";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP } from "@/pages/observability/components/runs/constants";
import type { ObservabilityRunMetric } from "@/pages/observability/components/runs/types";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";

const createStatusComponents = (statuses: RunStatus[], maxValue: number) =>
  statuses.map((status) => ({
    key: String(status),
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
    value: Math.floor(Math.random() * maxValue) + 1,
    color: OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP[status],
  }));

const createHourlyChartGroups = (
  statuses: RunStatus[],
): BarChartGroupDatum<ObservabilityRunMetric>[] =>
  Array.from({ length: 24 }, (_, i) => ({
    label: `${i.toString().padStart(2, "0")}:00`,
    bars: [{ metric: "runs" as const, components: createStatusComponents(statuses, 20) }],
  }));

const createDailyChartGroups = (
  days: number,
  statuses: RunStatus[],
): BarChartGroupDatum<ObservabilityRunMetric>[] => {
  const today = new Date();
  return Array.from({ length: days }, (_, i) => {
    const date = new Date(today);
    date.setDate(date.getDate() - (days - 1 - i));
    const label = `${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")}`;
    return {
      label,
      bars: [{ metric: "runs" as const, components: createStatusComponents(statuses, 100) }],
    };
  });
};

export const OBSERVABILITY_TIMEFRAME_TO_CHART_GROUPS_MAP: Record<
  ObservabilityTimeframe,
  (statuses: RunStatus[]) => BarChartGroupDatum<ObservabilityRunMetric>[]
> = {
  [ObservabilityTimeframe.TWENTY_FOUR_HOURS]: createHourlyChartGroups,
  [ObservabilityTimeframe.SEVEN_DAYS]: (statuses) => createDailyChartGroups(7, statuses),
  [ObservabilityTimeframe.THIRTY_DAYS]: (statuses) => createDailyChartGroups(30, statuses),
};
