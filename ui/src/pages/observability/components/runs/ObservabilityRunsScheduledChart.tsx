import { useMemo } from "react";

import { useSearch } from "@tanstack/react-router";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import {
  OBSERVABILITY_RUNS_CHART_HEIGHT,
  OBSERVABILITY_RUNS_SCHEDULED_INPUT,
  OBSERVABILITY_RUNS_SCHEDULED_SERIES,
} from "@/pages/observability/components/runs/constants";
import { createScheduledRunsChartGroups } from "@/pages/observability/components/runs/utils";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { useBucketLabelFormatter } from "@/pages/observability/utils";

import { useListRunsQuery } from "@/api/queries/runs";

const ObservabilityRunsScheduledChart = () => {
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_main/observability",
  });

  const bucketLabelFormatter = useBucketLabelFormatter(timeframe);

  const { data, isLoading } = useListRunsQuery({ input: OBSERVABILITY_RUNS_SCHEDULED_INPUT });

  const groups = useMemo(
    () => createScheduledRunsChartGroups(data?.runs ?? [], timeframe),
    [data, timeframe],
  );

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      padding={"24px 12px"}
      height={OBSERVABILITY_RUNS_CHART_HEIGHT}
      fillWidth
    >
      <BarChart
        series={OBSERVABILITY_RUNS_SCHEDULED_SERIES}
        groups={groups}
        labelFormatter={bucketLabelFormatter}
        isLoading={isLoading}
        fillWidth
        fillHeight
      />
    </FlexWrapper>
  );
};

export default ObservabilityRunsScheduledChart;
