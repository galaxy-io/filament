import { type FC, useMemo } from "react";

import LineChart, { type LineChartLineDatum } from "@galaxy-io/dls/charts/LineChart";
import type { ChartValueFormatter } from "@galaxy-io/dls/charts/types";
import Flex from "@galaxy-io/dls/layout/Flex";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { type Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_CHART_HEIGHT } from "@/pages/observability/components/band/constants";
import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import { useObservabilityBucketStart } from "@/pages/observability/hooks/useObservabilityBucketStart";
import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";
import {
  createObservabilityTimeseriesInput,
  formatBucketKey,
  getBucketLabelFormatter,
} from "@/pages/observability/utils";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";

interface ObservabilityBandTimeseriesChartProps {
  label: string;
  metric: Metric;
  color: RoleColor;
  valueFormatter: ChartValueFormatter;
}

const ObservabilityBandTimeseriesChart: FC<ObservabilityBandTimeseriesChartProps> = ({
  label,
  metric,
  color,
  valueFormatter,
}) => {
  const refetchInterval = useObservabilityRefetchInterval();
  const { timeframe } = useObservabilitySearch();
  const bucketStartMs = useObservabilityBucketStart(timeframe);

  const input = useMemo(
    () =>
      createObservabilityTimeseriesInput(timeframe, bucketStartMs, {
        metrics: [metric],
        groupBy: MetricDimension.UNSPECIFIED,
      }),
    [timeframe, bucketStartMs, metric],
  );

  const { data, isFetching } = useQueryTimeseriesQuery({
    input,
    options: { refetchInterval },
  });

  const lines = useMemo<LineChartLineDatum<string>[]>(
    () => [
      {
        metric: label,
        showArea: true,
        points: (data?.series[0]?.points ?? []).map((point) => ({
          x: formatBucketKey(point.bucketStartMs),
          y: point.values[0],
        })),
      },
    ],
    [data, label],
  );

  return (
    <Flex height={OBSERVABILITY_BAND_CHART_HEIGHT} fillWidth>
      <LineChart<string>
        series={{ [label]: { label, color } }}
        lines={lines}
        valueFormatter={valueFormatter}
        labelFormatter={getBucketLabelFormatter(
          OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe].granularity,
        )}
        isLoading={isFetching}
        hasLegend={false}
        ariaLabel={`${label} over time`}
      />
    </Flex>
  );
};

export default ObservabilityBandTimeseriesChart;
