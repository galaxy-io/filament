import type { BarChartGroupDatum } from "@galaxy-io/dls/charts/types";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  OBSERVABILITY_MOCK_PIPELINE_NAMES,
  OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP,
  OBSERVABILITY_RUN_STATUSES,
} from "@/pages/observability/components/runs/constants";
import type {
  ObservabilityMockRunInfo,
  ObservabilityRunMetric,
} from "@/pages/observability/components/runs/types";
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

export const createMockRuns = (): ObservabilityMockRunInfo[] => {
  const now = Date.now();
  const runs: ObservabilityMockRunInfo[] = [];

  for (let i = 0; i < 50; i++) {
    const startedAt = now - Math.floor(Math.random() * 24 * 60 * 60 * 1000);
    const duration = Math.floor(Math.random() * 30 * 60 * 1000) + 1000;
    const status =
      OBSERVABILITY_RUN_STATUSES[Math.floor(Math.random() * OBSERVABILITY_RUN_STATUSES.length)];
    const isTerminal = [RunStatus.COMPLETED, RunStatus.FAILED, RunStatus.CANCELED].includes(status);

    runs.push({
      runId: `run_${Math.random().toString(36).substring(2, 15)}`,
      pipelineId: `pipe_${Math.random().toString(36).substring(2, 10)}`,
      pipelineName:
        OBSERVABILITY_MOCK_PIPELINE_NAMES[
          Math.floor(Math.random() * OBSERVABILITY_MOCK_PIPELINE_NAMES.length)
        ],
      status,
      startedAt: BigInt(startedAt),
      endedAt: isTerminal ? BigInt(startedAt + duration) : BigInt(0),
      records: BigInt(Math.floor(Math.random() * 100000)),
      bytes: BigInt(Math.floor(Math.random() * 100000000)),
      error: status === RunStatus.FAILED ? "Connection timeout" : "",
    });
  }

  return runs.sort((a, b) => Number(b.startedAt - a.startedAt));
};
