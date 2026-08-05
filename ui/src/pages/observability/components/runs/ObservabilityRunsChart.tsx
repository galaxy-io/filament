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
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/utils";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

interface ObservabilityRunsChartProps {
  timeframe: ObservabilityTimeframe;
  statuses: RunStatus[];
}

const ObservabilityRunsChart = ({ timeframe, statuses }: ObservabilityRunsChartProps) => {
  const input = useMemo(
    () => createRunCountTimeseriesInput(timeframe, statuses),
    [timeframe, statuses],
  );

  const { data, isLoading } = useQueryTimeseriesQuery({ input });

  const groups = useMemo(() => {
    if (!statuses.length) {
      return [];
    }
    const { formatBucketLabel } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
    return mapTimeseriesToChartGroups(data?.series ?? [], formatBucketLabel);
  }, [data, statuses.length, timeframe]);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} padding={"24px 12px"} height={250} fillWidth>
      <BarChart
        series={OBSERVABILITY_RUNS_SERIES}
        groups={groups}
        isLoading={isLoading}
        fillWidth
        fillHeight
      />
    </FlexWrapper>
  );
};

export default ObservabilityRunsChart;
