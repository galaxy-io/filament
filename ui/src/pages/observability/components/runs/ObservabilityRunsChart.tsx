import { type FC, useMemo } from "react";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import type { ChartSelection, ChartSelectionInput } from "@galaxy-io/dls/charts/types";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import {
  OBSERVABILITY_RUNS_CHART_HEIGHT,
  OBSERVABILITY_RUNS_CHART_MIN_SEGMENT_LENGTH,
  OBSERVABILITY_RUNS_NO_STATUSES_PLACEHOLDER,
  OBSERVABILITY_RUNS_SERIES,
} from "@/pages/observability/components/runs/constants";
import {
  createRunCountTimeseriesInput,
  mapChartSelectionToRunsFilter,
  mapTimeseriesToChartGroups,
} from "@/pages/observability/components/runs/utils";
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import { getBucketLabelFormatter } from "@/pages/observability/utils";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

const ObservabilityRunsChart: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const { timeframe, statuses, runsBucket, runsStatus } = useObservabilitySearch();

  const bucketStartMs = useObservabilityBucketStart(timeframe);

  const bucketLabelFormatter = getBucketLabelFormatter(
    OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
  );

  const input = useMemo(
    () => createRunCountTimeseriesInput(timeframe, bucketStartMs, statuses),
    [timeframe, bucketStartMs, statuses],
  );

  const { data, isLoading } = useQueryTimeseriesQuery({
    input,
    options: { enabled: statuses.length > 0 },
  });

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
    void updateSearch((prev) => ({ ...prev, ...filter }));
  };

  return (
    <Flex
      alignItems={AlignItems.START}
      direction={FlexDirection.COLUMN}
      padding={[12, 16]}
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
        placeholder={statuses.length ? undefined : OBSERVABILITY_RUNS_NO_STATUSES_PLACEHOLDER}
        ariaLabel="Runs by status"
        minSegmentLength={OBSERVABILITY_RUNS_CHART_MIN_SEGMENT_LENGTH}
      />
    </Flex>
  );
};

export default ObservabilityRunsChart;
