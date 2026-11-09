import { type FC, useMemo } from "react";

import LineChart from "@galaxy-io/dls/charts/LineChart";
import { ChartSwatch, type ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import Flex from "@galaxy-io/dls/layout/Flex";

import {
  type Metric,
  MetricDimension,
  type QueryAggregateRequest,
} from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_CHART_HEIGHT } from "@/pages/observability/components/band/constants";
import { mapObservabilityBandPipelineLines } from "@/pages/observability/components/band/utils";
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import { useObservabilityBandRanking } from "@/pages/observability/hooks/useObservabilityBandRanking";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";
import {
  createObservabilityTimeseriesInput,
  getBucketLabelFormatter,
} from "@/pages/observability/utils";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

interface ObservabilityBandPipelinesChartProps {
  label: string;
  metric: Metric;
  rankingInput: QueryAggregateRequest;
  valueFormatter: ChartValueFormatter;
}

const ObservabilityBandPipelinesChart: FC<ObservabilityBandPipelinesChartProps> = ({
  label,
  metric,
  rankingInput,
  valueFormatter,
}) => {
  const refetchInterval = useObservabilityRefetchInterval();
  const { timeframe } = useObservabilitySearch();
  const bucketStartMs = useObservabilityBucketStart(timeframe);
  const { ranking } = useObservabilityBandRanking(rankingInput);

  const input = useMemo(
    () =>
      createObservabilityTimeseriesInput(timeframe, bucketStartMs, {
        metrics: [metric],
        groupBy: MetricDimension.PIPELINE_ID,
      }),
    [timeframe, bucketStartMs, metric],
  );

  const { data, isFetching } = useQueryTimeseriesQuery({
    input,
    options: { refetchInterval },
  });

  const { series, lines } = useMemo(
    () => mapObservabilityBandPipelineLines(ranking, data?.series ?? []),
    [ranking, data],
  );

  return (
    <Flex height={OBSERVABILITY_BAND_CHART_HEIGHT} fillWidth>
      <LineChart<string>
        series={series}
        lines={lines}
        valueFormatter={valueFormatter}
        labelFormatter={getBucketLabelFormatter(
          OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
        )}
        isLoading={isFetching}
        swatch={ChartSwatch.SQUARE}
        hasLegend={false}
        ariaLabel={`${label} per pipeline over time`}
      />
    </Flex>
  );
};

export default ObservabilityBandPipelinesChart;
