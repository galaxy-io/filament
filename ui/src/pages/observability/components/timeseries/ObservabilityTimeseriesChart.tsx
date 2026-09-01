import { useMemo } from "react";

import { useSearch } from "@tanstack/react-router";

import LineChart from "@galaxy-io/dls/charts/LineChart";
import type {
  ChartPalette,
  ChartValueFormatter,
  LineChartCurve,
  LineChartLineDatum,
} from "@galaxy-io/dls/charts/types";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { type Metric, MetricDimension, type Timeseries } from "@/gen/metrics/v1/metrics_pb";

import {
  OBSERVABILITY_TIMESERIES_CHART_HEIGHT,
  OBSERVABILITY_TIMESERIES_PIVOT_PALETTE,
} from "@/pages/observability/components/timeseries/constants";
import { OBSERVABILITY_PIPELINES_INPUT } from "@/pages/observability/constants";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createObservabilityTimeseriesInput,
  formatBucketKey,
  useBucketLabelFormatter,
} from "@/pages/observability/utils";
import {
  PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/pages/pipelines/history/constants";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";
import { useListPipelinesQuery } from "@/api/queries/pipelines";

interface ObservabilityTimeseriesChartProps {
  seriesLabel: string;
  metric: Metric;
  color: ChartPalette;
  pivot: MetricDimension | undefined;
  curve: LineChartCurve;
  valueFormatter?: ChartValueFormatter;
}

const ObservabilityTimeseriesChart = ({
  seriesLabel,
  metric,
  color,
  pivot,
  curve,
  valueFormatter,
}: ObservabilityTimeseriesChartProps) => {
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_app/_main/observability",
  });

  const pivotDimension = pivot ?? MetricDimension.UNSPECIFIED;

  const bucketLabelFormatter = useBucketLabelFormatter(timeframe);

  const input = useMemo(
    () =>
      createObservabilityTimeseriesInput(timeframe, {
        metrics: [metric],
        groupBy: pivotDimension,
      }),
    [timeframe, metric, pivotDimension],
  );

  const { data, isLoading } = useQueryTimeseriesQuery({ input });
  const { data: pipelinesData } = useListPipelinesQuery({
    input: OBSERVABILITY_PIPELINES_INPUT,
  });

  const { series, lines } = useMemo(() => {
    const timeseries = data?.series ?? [];
    const pipelineNamesByPipelineId = new Map(
      (pipelinesData?.pipelines ?? []).map((pipeline) => [
        pipeline.id,
        formatPipelineName(pipeline, true),
      ]),
    );

    const keyToLabel = (key: Timeseries["key"]) => {
      if (pivotDimension === MetricDimension.STATUS) {
        return PIPELINE_RUN_STATUS_TO_LABEL_MAP[Number(key) as RunStatus];
      }
      if (pivotDimension === MetricDimension.PIPELINE_ID) {
        return pipelineNamesByPipelineId.get(key) ?? key;
      }
      return seriesLabel;
    };

    const keyToColor = (key: Timeseries["key"], index: number) => {
      if (pivotDimension === MetricDimension.STATUS) {
        return PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP[Number(key) as RunStatus];
      }
      if (pivotDimension === MetricDimension.PIPELINE_ID) {
        return OBSERVABILITY_TIMESERIES_PIVOT_PALETTE[
          index % OBSERVABILITY_TIMESERIES_PIVOT_PALETTE.length
        ];
      }
      return color;
    };

    return {
      series: Object.fromEntries(
        timeseries.map((keySeries, index) => [
          keySeries.key || seriesLabel,
          {
            label: keyToLabel(keySeries.key),
            color: keyToColor(keySeries.key, index),
          },
        ]),
      ),
      lines: timeseries.map<LineChartLineDatum<string>>((keySeries) => ({
        metric: keySeries.key || seriesLabel,
        showArea: pivotDimension === MetricDimension.UNSPECIFIED,
        points: keySeries.points.map((point) => ({
          x: formatBucketKey(point.bucketStartMs),
          y: point.values[0],
        })),
      })),
    };
  }, [data, pipelinesData, pivotDimension, seriesLabel, color]);

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      padding={"16px 12px"}
      height={OBSERVABILITY_TIMESERIES_CHART_HEIGHT}
      fillWidth
    >
      <LineChart<string>
        series={series}
        lines={lines}
        curve={curve}
        valueFormatter={valueFormatter}
        labelFormatter={bucketLabelFormatter}
        tooltipMaxItems={8}
        isLoading={isLoading}
        fillWidth
        fillHeight
        showGrid
        showLegend
      />
    </FlexWrapper>
  );
};

export default ObservabilityTimeseriesChart;
