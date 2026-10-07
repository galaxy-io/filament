import { useNavigate, useSearch } from "@tanstack/react-router";

import { ChartCurve } from "@galaxy-io/dls/charts/types";
import ToggleInput, {
  ToggleInputSize,
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";
import Widget from "@galaxy-io/dls/widget/Widget";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import type { ObservabilityChartView } from "@/pages/observability/components/timeseries/constants";
import ObservabilityPivotSelect from "@/pages/observability/components/timeseries/ObservabilityPivotSelect";
import ObservabilityTimeseriesChart from "@/pages/observability/components/timeseries/ObservabilityTimeseriesChart";

interface ObservabilityTimeseriesWidgetProps<View extends string> {
  views: Record<View, ObservabilityChartView>;
  defaultView: View;
  defaultPivot?: MetricDimension;
  viewSearchKey: "throughput" | "usage";
  pivotSearchKey: "throughputPivot" | "usagePivot";
}

const ObservabilityTimeseriesWidget = <View extends string>({
  views,
  defaultView,
  defaultPivot,
  viewSearchKey,
  pivotSearchKey,
}: ObservabilityTimeseriesWidgetProps<View>) => {
  const navigate = useNavigate();
  const search = useSearch({ from: "/_app/_main/observability" });

  const view = (search[viewSearchKey] as View | undefined) ?? defaultView;
  const searchPivot = search[pivotSearchKey];
  const pivot =
    searchPivot === MetricDimension.UNSPECIFIED ? undefined : (searchPivot ?? defaultPivot);

  const { label, seriesLabel, metric, color, valueFormatter } = views[view];

  const handleViewChange = (nextView: View) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, [viewSearchKey]: nextView }),
    });
  };

  const handlePivotChange = (nextPivot: MetricDimension | undefined) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, [pivotSearchKey]: nextPivot ?? MetricDimension.UNSPECIFIED }),
    });
  };

  const switcherItems: ToggleOption<View>[] = (
    Object.entries(views) as [View, ObservabilityChartView][]
  ).map(([id, viewConfig]) => ({
    id,
    label: viewConfig.label,
  }));

  return (
    <Widget
      isFlush
      gap={0}
      header={label}
      actions={
        <>
          <ToggleInput
            size={ToggleInputSize.SMALL}
            variant={ToggleInputVariant.PRIMARY}
            options={switcherItems}
            value={view}
            onChange={handleViewChange}
          />
          <ObservabilityPivotSelect value={pivot} onChange={handlePivotChange} />
        </>
      }
    >
      <ObservabilityTimeseriesChart
        seriesLabel={seriesLabel}
        metric={metric}
        color={color}
        pivot={pivot}
        curve={ChartCurve.LINEAR}
        valueFormatter={valueFormatter}
      />
    </Widget>
  );
};

export default ObservabilityTimeseriesWidget;
