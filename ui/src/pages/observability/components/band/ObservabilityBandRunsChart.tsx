import type { FC } from "react";

import BarChart, { type BarChartGroupDatum } from "@galaxy-io/dls/charts/BarChart";
import { ChartSwatch } from "@galaxy-io/dls/charts/types";
import Flex from "@galaxy-io/dls/layout/Flex";

import {
  OBSERVABILITY_BAND_CHART_HEIGHT,
  OBSERVABILITY_BAND_RUNS_MIN_SEGMENT_LENGTH,
  OBSERVABILITY_BAND_RUNS_SERIES,
} from "@/pages/observability/components/band/constants";
import type { ObservabilityBandRunsMetric } from "@/pages/observability/components/band/types";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";

interface ObservabilityBandRunsChartProps {
  groups: BarChartGroupDatum<ObservabilityBandRunsMetric>[];
  isLoading: boolean;
  ariaLabel: string;
}

const ObservabilityBandRunsChart: FC<ObservabilityBandRunsChartProps> = ({
  groups,
  isLoading,
  ariaLabel,
}) => {
  const { labelFormatter } = useObservabilityMetricInputs();

  return (
    <Flex height={OBSERVABILITY_BAND_CHART_HEIGHT} fillWidth>
      <BarChart
        series={OBSERVABILITY_BAND_RUNS_SERIES}
        groups={groups}
        labelFormatter={labelFormatter}
        isLoading={isLoading}
        swatch={ChartSwatch.SQUARE}
        hasLegend={false}
        minSegmentLength={OBSERVABILITY_BAND_RUNS_MIN_SEGMENT_LENGTH}
        ariaLabel={ariaLabel}
      />
    </Flex>
  );
};

export default ObservabilityBandRunsChart;
