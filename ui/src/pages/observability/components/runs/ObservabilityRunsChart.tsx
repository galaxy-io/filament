import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import type { ChartSelectionEvent, ChartSelectionInput } from "@galaxy-io/dls/charts/types";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import {
  OBSERVABILITY_RUNS_CHART_HEIGHT,
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

  const selection = useMemo<ChartSelectionInput>(
    () =>
      runsBucket === undefined
        ? null
        : {
            categoryKey: String(runsBucket),
            seriesKey: runsStatus === undefined ? undefined : String(runsStatus),
          },
    [runsBucket, runsStatus],
  );

  const handleSelect = (event: ChartSelectionEvent) => {
    const selected = mapChartSelectionToRunsFilter(event);
    const isSelected = selected.runsBucket === runsBucket && selected.runsStatus === runsStatus;
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        runsBucket: isSelected ? undefined : selected.runsBucket,
        runsStatus: isSelected ? undefined : selected.runsStatus,
      }),
    });
  };

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      padding={"24px 12px"}
      height={OBSERVABILITY_RUNS_CHART_HEIGHT}
      fillWidth
    >
      <BarChart
        series={OBSERVABILITY_RUNS_SERIES}
        groups={groups}
        labelFormatter={bucketLabelFormatter}
        selection={selection}
        onSelect={handleSelect}
        isLoading={isLoading}
        fillWidth
        fillHeight
      />
    </FlexWrapper>
  );
};

export default ObservabilityRunsChart;
