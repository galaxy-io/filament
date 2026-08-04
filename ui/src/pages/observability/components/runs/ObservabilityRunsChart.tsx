import { useMemo } from "react";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_RUNS_SERIES } from "@/pages/observability/components/runs/constants";
import { OBSERVABILITY_TIMEFRAME_TO_CHART_GROUPS_MAP } from "@/pages/observability/components/runs/utils";
import type { ObservabilityTimeframe } from "@/pages/observability/types";

interface ObservabilityRunsChartProps {
  timeframe: ObservabilityTimeframe;
  statuses: RunStatus[];
}

const ObservabilityRunsChart = ({ timeframe, statuses }: ObservabilityRunsChartProps) => {
  const groups = useMemo(
    () => (statuses.length ? OBSERVABILITY_TIMEFRAME_TO_CHART_GROUPS_MAP[timeframe](statuses) : []),
    [timeframe, statuses],
  );

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} padding={"24px 12px"} height={250} fillWidth>
      <BarChart series={OBSERVABILITY_RUNS_SERIES} groups={groups} fillWidth fillHeight />
    </FlexWrapper>
  );
};

export default ObservabilityRunsChart;
