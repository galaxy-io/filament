import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useSearch } from "@tanstack/react-router";

import LineChart from "@galaxy-io/dls/charts/LineChart";
import type {
  ChartPalette,
  ChartValueFormatter,
  LineChartLineDatum,
} from "@galaxy-io/dls/charts/types";
import { LineChartCurve } from "@galaxy-io/dls/charts/types";
import FlexWrapper, {
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import SelectInput, {
  type SelectInputOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import {
  type Metric,
  MetricDimension,
  QueryTimeseriesRequestSchema,
} from "@/gen/metrics/v1/metrics_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import { OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP } from "@/pages/observability/components/runs/constants";
import {
  METRIC_DIMENSION_PIVOT_OPTIONS,
  OBSERVABILITY_TIMESERIES_PIVOT_PALETTE,
} from "@/pages/observability/components/timeseries/constants";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createTimeframeWindow,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
} from "@/pages/observability/utils";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";
import { formatPipelineName } from "@/pages/pipelines/utils";

import type { ObservabilityPivotDimension } from "@/routes/_main/observability";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";
import { useListPipelinesQuery } from "@/api/queries/pipelines";

interface ObservabilityTimeseriesWidgetProps {
  title: string;
  seriesLabel: string;
  metric: Metric;
  color: ChartPalette;
  pivot: ObservabilityPivotDimension | undefined;
  onPivotChange: (pivot: ObservabilityPivotDimension | null) => void;
  valueFormatter?: ChartValueFormatter;
}

const ObservabilityTimeseriesWidget = ({
  title,
  seriesLabel,
  metric,
  color,
  pivot,
  onPivotChange,
  valueFormatter,
}: ObservabilityTimeseriesWidgetProps) => {
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_main/observability",
  });

  const pivotDimension = pivot ?? MetricDimension.UNSPECIFIED;

  const selectedPivotOption =
    METRIC_DIMENSION_PIVOT_OPTIONS.find((option) => option.value === pivot) ??
    null;

  const handlePivotChange = (option: SelectInputOption) => {
    onPivotChange(option.value as ObservabilityPivotDimension);
  };

  const handlePivotReset = () => {
    onPivotChange(null);
  };

  const input = useMemo(() => {
    const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
    const { sinceMs, untilMs } = createTimeframeWindow(timeframe);
    return create(QueryTimeseriesRequestSchema, {
      metrics: [metric],
      sinceMs,
      untilMs,
      granularity,
      tzOffsetMinutes: -new Date().getTimezoneOffset(),
      groupBy: pivotDimension,
    });
  }, [timeframe, metric, pivotDimension]);

  const { data, isLoading } = useQueryTimeseriesQuery({ input });
  const { data: pipelinesData } = useListPipelinesQuery();

  const { series, lines } = useMemo(() => {
    const { formatBucketLabel } =
      OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
    const timeseries = data?.series ?? [];
    const pipelineNamesByPipelineId = new Map(
      (pipelinesData?.pipelines ?? []).map((pipeline) => [
        pipeline.id,
        formatPipelineName(pipeline),
      ]),
    );

    const keyToLabel = (key: string) => {
      if (pivotDimension === MetricDimension.STATUS) {
        return PIPELINE_RUN_STATUS_TO_LABEL_MAP[Number(key) as RunStatus];
      }
      if (pivotDimension === MetricDimension.PIPELINE_ID) {
        return pipelineNamesByPipelineId.get(key) ?? key;
      }
      return seriesLabel;
    };

    const keyToColor = (key: string, index: number) => {
      if (pivotDimension === MetricDimension.STATUS) {
        return OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP[Number(key) as RunStatus];
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
          x: formatBucketLabel(point.bucketStartMs),
          y: point.values[0],
        })),
      })),
    };
  }, [data, pipelinesData, pivotDimension, timeframe, seriesLabel, color]);

  return (
    <Widget fillWidth fillHeight noPadding>
      <BaseToolbar
        leadingActions={[
          <Text
            key="title"
            variant={TextVariant.PRIMARY}
            weight={TextWeight.MEDIUM}
          >
            {title}
          </Text>,
        ]}
        trailingActions={[
          <SelectInput
            key="pivot-selector"
            options={METRIC_DIMENSION_PIVOT_OPTIONS}
            value={selectedPivotOption}
            onChange={handlePivotChange}
            onReset={handlePivotReset}
            placeholder="Pivot"
            width={140}
          />,
        ]}
      />
      <HorizontalDivider />
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        padding={"16px 12px"}
        height={240}
        fillWidth
      >
        <LineChart<string>
          series={series}
          lines={lines}
          curve={LineChartCurve.LINEAR}
          valueFormatter={valueFormatter}
          isLoading={isLoading}
          fillWidth
          fillHeight
          showGrid
          showLegend
        />
      </FlexWrapper>
    </Widget>
  );
};

export default ObservabilityTimeseriesWidget;
