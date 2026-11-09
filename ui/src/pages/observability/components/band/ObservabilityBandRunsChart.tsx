import { type FC, useMemo } from "react";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import { ChartSwatch } from "@galaxy-io/dls/charts/types";
import Flex from "@galaxy-io/dls/layout/Flex";

import {
  OBSERVABILITY_BAND_CHART_HEIGHT,
  OBSERVABILITY_BAND_RUNS_MIN_SEGMENT_LENGTH,
  OBSERVABILITY_BAND_RUNS_SERIES,
} from "@/pages/observability/components/band/constants";
import {
  createObservabilityBandRunsInput,
  mapObservabilityBandRunsGroups,
} from "@/pages/observability/components/band/utils";
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";
import { getBucketLabelFormatter } from "@/pages/observability/utils";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

const ObservabilityBandRunsChart: FC = () => {
  const { timeframe } = useObservabilitySearch();
  const bucketStartMs = useObservabilityBucketStart(timeframe);
  const refetchInterval = useObservabilityRefetchInterval();

  const input = useMemo(
    () => createObservabilityBandRunsInput(timeframe, bucketStartMs),
    [timeframe, bucketStartMs],
  );

  const { data, isFetching } = useQueryTimeseriesQuery({ input, options: { refetchInterval } });

  const groups = useMemo(() => mapObservabilityBandRunsGroups(data?.series ?? []), [data]);

  return (
    <Flex height={OBSERVABILITY_BAND_CHART_HEIGHT} fillWidth>
      <BarChart
        series={OBSERVABILITY_BAND_RUNS_SERIES}
        groups={groups}
        labelFormatter={getBucketLabelFormatter(
          OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
        )}
        isLoading={isFetching}
        swatch={ChartSwatch.SQUARE}
        hasLegend={false}
        minSegmentLength={OBSERVABILITY_BAND_RUNS_MIN_SEGMENT_LENGTH}
        ariaLabel="Runs by status"
      />
    </Flex>
  );
};

export default ObservabilityBandRunsChart;
