import { useNavigate, useSearch } from "@tanstack/react-router";

import { LineChartCurve } from "@galaxy-io/dls/charts/types";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

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
  const pivot = search[pivotSearchKey] ?? defaultPivot;

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

  const switcherItems: SwitcherInputItem[] = (
    Object.entries(views) as [View, ObservabilityChartView][]
  ).map(([id, viewConfig]) => ({
    id,
    label: viewConfig.label,
    onClick: () => handleViewChange(id),
  }));

  return (
    <Widget fillWidth fillHeight noPadding>
      <BaseToolbar
        leadingActions={[
          <Text key="title" weight={TextWeight.MEDIUM}>
            {label}
          </Text>,
        ]}
        trailingActions={[
          <SwitcherInput
            key="view-switcher"
            variant={InputVariant.TERTIARY}
            items={switcherItems}
            selectedId={view}
          />,
          <ObservabilityPivotSelect
            key="pivot-selector"
            value={pivot}
            onChange={handlePivotChange}
          />,
        ]}
      />
      <HorizontalDivider />
      <ObservabilityTimeseriesChart
        seriesLabel={seriesLabel}
        metric={metric}
        color={color}
        pivot={pivot}
        curve={LineChartCurve.LINEAR}
        valueFormatter={valueFormatter}
      />
    </Widget>
  );
};

export default ObservabilityTimeseriesWidget;
