import { useMemo } from "react";

import { useSearch } from "@tanstack/react-router";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import {
  OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  OBSERVABILITY_RUNS_SERIES,
} from "@/pages/observability/components/runs/constants";
import {
  createRunCountTimeseriesInput,
  mapTimeseriesToChartGroups,
} from "@/pages/observability/components/runs/utils";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { useBucketLabelFormatter } from "@/pages/observability/utils";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

const ObservabilityRunsChart = () => {
  const {
    timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  } = useSearch({ from: "/_main/observability" });

  const bucketLabelFormatter = useBucketLabelFormatter(timeframe);

  const input = useMemo(
    () => createRunCountTimeseriesInput(timeframe, statuses),
    [timeframe, statuses],
  );

  const { data, isLoading } = useQueryTimeseriesQuery({ input });

  const groups = useMemo(() => {
    if (!statuses.length) {
      return [];
    }
    return mapTimeseriesToChartGroups(data?.series ?? []);
  }, [data, statuses.length]);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} padding={"24px 12px"} height={250} fillWidth>
      <BarChart
        series={OBSERVABILITY_RUNS_SERIES}
        groups={groups}
        labelFormatter={bucketLabelFormatter}
        isLoading={isLoading}
        fillWidth
        fillHeight
      />
    </FlexWrapper>
  );
};

export default ObservabilityRunsChart;
