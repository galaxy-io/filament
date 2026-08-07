import { useMemo } from "react";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_RUNS_SERIES } from "@/pages/observability/components/runs/constants";
import {
  createRunCountTimeseriesInput,
  mapTimeseriesToChartGroups,
} from "@/pages/observability/components/runs/utils";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import { useBucketLabelFormatter, useSlidingTimeframeWindow } from "@/pages/observability/utils";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

interface ObservabilityRunsChartProps {
  timeframe: ObservabilityTimeframe;
  statuses: RunStatus[];
}

const ObservabilityRunsChart = ({ timeframe, statuses }: ObservabilityRunsChartProps) => {
  const timeframeWindow = useSlidingTimeframeWindow(timeframe);
  const bucketLabelFormatter = useBucketLabelFormatter(timeframe);

  const input = useMemo(
    () => createRunCountTimeseriesInput(timeframeWindow, timeframe, statuses),
    [timeframeWindow, timeframe, statuses],
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
