import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import type { ObservabilityChartView } from "@/pages/observability/components/timeseries/constants";
import ObservabilityPivotSelect from "@/pages/observability/components/timeseries/ObservabilityPivotSelect";
import ObservabilityTimeseriesChart from "@/pages/observability/components/timeseries/ObservabilityTimeseriesChart";

interface ObservabilityTimeseriesWidgetProps<View extends string> {
  views: Record<View, ObservabilityChartView>;
  view: View;
  onViewChange: (view: View) => void;
  pivot: MetricDimension | undefined;
  onPivotChange: (pivot: MetricDimension | undefined) => void;
}

const ObservabilityTimeseriesWidget = <View extends string>({
  views,
  view,
  onViewChange,
  pivot,
  onPivotChange,
}: ObservabilityTimeseriesWidgetProps<View>) => {
  const { label, seriesLabel, metric, color, valueFormatter } = views[view];

  const switcherItems: SwitcherInputItem[] = (
    Object.entries(views) as [View, ObservabilityChartView][]
  ).map(([id, viewConfig]) => ({
    id,
    label: viewConfig.label,
    onClick: () => onViewChange(id),
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
          <SwitcherInput key="view-switcher" items={switcherItems} selectedId={view} />,
          <ObservabilityPivotSelect key="pivot-selector" value={pivot} onChange={onPivotChange} />,
        ]}
      />
      <HorizontalDivider />
      <ObservabilityTimeseriesChart
        seriesLabel={seriesLabel}
        metric={metric}
        color={color}
        pivot={pivot}
        valueFormatter={valueFormatter}
      />
    </Widget>
  );
};

export default ObservabilityTimeseriesWidget;
