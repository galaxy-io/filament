import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useSearch } from "@tanstack/react-router";

import LineChart from "@galaxy-io/dls/charts/LineChart";
import type { LineChartLineDatum } from "@galaxy-io/dls/charts/types";
import { LineChartCurve } from "@galaxy-io/dls/charts/types";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { MetricDimension, QueryTimeseriesRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import { OBSERVABILITY_RUN_STATUS_TO_COLOR_MAP } from "@/pages/observability/components/runs/constants";
import {
  METRIC_DIMENSION_PIVOT_OPTIONS,
  OBSERVABILITY_METRIC_VIEW_TO_CONFIG_MAP,
  OBSERVABILITY_TIMESERIES_PIVOT_PALETTE,
} from "@/pages/observability/components/timeseries/constants";
import ObservabilityMetricViewSwitcher from "@/pages/observability/components/timeseries/ObservabilityMetricViewSwitcher";
import { OBSERVABILITY_PIPELINES_INPUT } from "@/pages/observability/constants";
import { type ObservabilityMetricView, ObservabilityTimeframe } from "@/pages/observability/types";
import {
  createTimeframeSince,
  formatBucketKey,
  OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP,
  useBucketLabelFormatter,
} from "@/pages/observability/utils";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useQueryTimeseriesQuery } from "@/api/queries/metrics";
import { useListPipelinesQuery } from "@/api/queries/pipelines";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";

interface ObservabilityTimeseriesWidgetProps {
  view: ObservabilityMetricView;
  onViewChange: (view: ObservabilityMetricView) => void;
  pivot: MetricDimension | undefined;
  onPivotChange: (pivot: MetricDimension | undefined) => void;
}

const ObservabilityTimeseriesWidget = ({
  view,
  onViewChange,
  pivot,
  onPivotChange,
}: ObservabilityTimeseriesWidgetProps) => {
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_main/observability",
  });

  const { seriesLabel, metric, color, valueFormatter } =
    OBSERVABILITY_METRIC_VIEW_TO_CONFIG_MAP[view];

  const pivotDimension = pivot ?? MetricDimension.UNSPECIFIED;

  const selectedPivotOption =
    METRIC_DIMENSION_PIVOT_OPTIONS.find((option) => option.value === pivot) ?? null;

  const handlePivotChange = (option: SelectInputOption) => {
    onPivotChange(option.value as MetricDimension);
  };

  const handlePivotReset = () => {
    onPivotChange(undefined);
  };

  const bucketLabelFormatter = useBucketLabelFormatter(timeframe);

  const input = useMemo(() => {
    const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];
    return create(QueryTimeseriesRequestSchema, {
      metrics: [metric],
      sinceMs: createTimeframeSince(timeframe),
      granularity,
      tzOffsetMinutes: -new Date().getTimezoneOffset(),
      groupBy: pivotDimension,
    });
  }, [timeframe, metric, pivotDimension]);

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
          x: formatBucketKey(point.bucketStartMs),
          y: point.values[0],
        })),
      })),
    };
  }, [data, pipelinesData, pivotDimension, seriesLabel, color]);

  return (
    <Widget fillWidth fillHeight noPadding>
      <BaseToolbar
        leadingActions={[
          <Text key="title" weight={TextWeight.MEDIUM}>
            {seriesLabel}
          </Text>,
        ]}
        trailingActions={[
          <ObservabilityMetricViewSwitcher
            key="metric-view-switcher"
            value={view}
            onChange={onViewChange}
          />,
          <SelectInput
            key="pivot-selector"
            options={METRIC_DIMENSION_PIVOT_OPTIONS}
            value={selectedPivotOption}
            onChange={handlePivotChange}
            onReset={handlePivotReset}
            placeholder="Pivot"
            width={150}
          />,
        ]}
      />
      <HorizontalDivider />
      <FlexWrapper direction={FlexDirection.COLUMN} padding={"16px 12px"} height={240} fillWidth>
        <LineChart<string>
          series={series}
          lines={lines}
          curve={LineChartCurve.LINEAR}
          valueFormatter={valueFormatter}
          labelFormatter={bucketLabelFormatter}
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
