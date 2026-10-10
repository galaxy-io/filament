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

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

type ObservabilityTimeseriesViewKey = "throughput" | "usage";

type ObservabilityTimeseriesView<TKey extends ObservabilityTimeseriesViewKey> = NonNullable<
  ObservabilitySearch[TKey]
>;

interface ObservabilityTimeseriesWidgetProps<TKey extends ObservabilityTimeseriesViewKey> {
  views: ObservabilityTimeseriesView<TKey>[];
  viewToConfigMap: Record<ObservabilityTimeseriesView<TKey>, ObservabilityChartView>;
  defaultView: ObservabilityTimeseriesView<TKey>;
  defaultPivot?: MetricDimension;
  viewSearchKey: TKey;
  pivotSearchKey: "throughputPivot" | "usagePivot";
}

const ObservabilityTimeseriesWidget = <TKey extends ObservabilityTimeseriesViewKey>({
  views,
  viewToConfigMap,
  defaultView,
  defaultPivot,
  viewSearchKey,
  pivotSearchKey,
}: ObservabilityTimeseriesWidgetProps<TKey>) => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const search = useObservabilitySearch();

  const view: ObservabilityTimeseriesView<TKey> = search[viewSearchKey] ?? defaultView;
  const searchPivot = search[pivotSearchKey];
  const pivot =
    searchPivot === MetricDimension.UNSPECIFIED ? undefined : (searchPivot ?? defaultPivot);

  const { label, seriesLabel, metric, color, valueFormatter } = viewToConfigMap[view];

  const handleViewChange = (nextView: ObservabilityTimeseriesView<TKey>) => {
    void updateSearch((prev) => ({ ...prev, [viewSearchKey]: nextView }));
  };

  const handlePivotChange = (nextPivot: MetricDimension | undefined) => {
    void updateSearch((prev) => ({
      ...prev,
      [pivotSearchKey]: nextPivot ?? MetricDimension.UNSPECIFIED,
    }));
  };

  const switcherItems: ToggleOption<ObservabilityTimeseriesView<TKey>>[] = views.map((id) => ({
    id,
    label: viewToConfigMap[id].label,
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
