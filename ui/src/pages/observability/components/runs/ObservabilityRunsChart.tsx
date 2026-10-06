import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import type { ChartSelection, ChartSelectionInput } from "@galaxy-io/dls/charts/types";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import {
  OBSERVABILITY_RUNS_CHART_HEIGHT,
  OBSERVABILITY_RUNS_CHART_MIN_SEGMENT_LENGTH,
  OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  OBSERVABILITY_RUNS_SERIES,
} from "@/pages/observability/components/runs/constants";
import {
  createRunCountTimeseriesInput,
  mapChartSelectionToRunsFilter,
  mapTimeseriesToChartGroups,
} from "@/pages/observability/components/runs/utils";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { useBucketLabelFormatter } from "@/pages/observability/utils";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

const ObservabilityRunsChart = () => {
  const navigate = useNavigate();
  const {
    timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
    runsBucket,
    runsStatus,
  } = useSearch({ from: "/_app/_main/observability" });

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

  const selection = useMemo<ChartSelectionInput>(
    () =>
      runsBucket === undefined && runsStatus === undefined
        ? null
        : {
            categoryKey: runsBucket === undefined ? undefined : String(runsBucket),
            seriesKey: runsStatus === undefined ? undefined : String(runsStatus),
          },
    [runsBucket, runsStatus],
  );

  const handleSelectionChange = (next: ChartSelection[]) => {
    const filter = mapChartSelectionToRunsFilter(next[next.length - 1]);
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, ...filter }),
    });
  };

  return (
    <Flex
      alignItems={AlignItems.START}
      direction={FlexDirection.COLUMN}
      padding={[24, 12]}
      height={OBSERVABILITY_RUNS_CHART_HEIGHT}
      fillWidth
    >
      <BarChart
        series={OBSERVABILITY_RUNS_SERIES}
        groups={groups}
        labelFormatter={bucketLabelFormatter}
        selection={selection}
        onSelectionChange={handleSelectionChange}
        isFilterable
        isLoading={isLoading}
        minSegmentLength={OBSERVABILITY_RUNS_CHART_MIN_SEGMENT_LENGTH}
      />
    </Flex>
  );
};

export default ObservabilityRunsChart;
