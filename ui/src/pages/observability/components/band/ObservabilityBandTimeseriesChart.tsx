import { type FC, useMemo } from "react";

import AreaChart from "@galaxy-io/dls/charts/AreaChart";
import LineChart from "@galaxy-io/dls/charts/LineChart";
import { ChartCurve, ChartSwatch, type ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import Flex from "@galaxy-io/dls/layout/Flex";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { type Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_CHART_HEIGHT } from "@/pages/observability/components/band/constants";
import { mapObservabilityBandLines } from "@/pages/observability/components/band/utils";
import { useObservabilityBandRanking } from "@/pages/observability/hooks/useObservabilityBandRanking";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";
import { useObservabilityTimeseriesQuery } from "@/pages/observability/hooks/useObservabilityTimeseriesQuery";

interface ObservabilityBandTimeseriesChartProps {
  label: string;
  metric: Metric;
  pivot: MetricDimension;
  color: RoleColor;
  valueFormatter: ChartValueFormatter;
  isStacked: boolean;
}

const ObservabilityBandTimeseriesChart: FC<ObservabilityBandTimeseriesChartProps> = ({
  label,
  metric,
  pivot,
  color,
  valueFormatter,
  isStacked,
}) => {
  const { createAggregateInput, createSeriesInput, labelFormatter } =
    useObservabilityMetricInputs();
  const { ranking } = useObservabilityBandRanking(
    createAggregateInput(metric, MetricDimension.PIPELINE_ID),
  );
  const { data, isFetching } = useObservabilityTimeseriesQuery(createSeriesInput(metric, pivot));

  const { series, lines } = useMemo(
    () => mapObservabilityBandLines(pivot, data?.series ?? [], ranking, label, color),
    [pivot, data, ranking, label, color],
  );

  const chartProps = {
    series,
    valueFormatter,
    labelFormatter,
    curve: ChartCurve.LINEAR,
    isLoading: isFetching,
    swatch: ChartSwatch.SQUARE,
    hasLegend: false,
    ariaLabel: `${label} over time`,
  };

  return (
    <Flex height={OBSERVABILITY_BAND_CHART_HEIGHT} fillWidth>
      {isStacked ? (
        <AreaChart<string> {...chartProps} areas={lines} isStacked />
      ) : (
        <LineChart<string> {...chartProps} lines={lines} />
      )}
    </Flex>
  );
};

export default ObservabilityBandTimeseriesChart;
