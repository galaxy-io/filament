import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";

import { OBSERVABILITY_METRIC_VIEW_TO_LABEL_MAP } from "@/pages/observability/components/timeseries/constants";
import { ObservabilityMetricView } from "@/pages/observability/types";

interface ObservabilityMetricViewSwitcherProps {
  value: ObservabilityMetricView;
  onChange: (view: ObservabilityMetricView) => void;
}

const ObservabilityMetricViewSwitcher = ({
  value,
  onChange,
}: ObservabilityMetricViewSwitcherProps) => {
  const items: SwitcherInputItem[] = Object.values(ObservabilityMetricView).map((view) => ({
    id: view,
    label: OBSERVABILITY_METRIC_VIEW_TO_LABEL_MAP[view],
    onClick: () => onChange(view),
  }));

  return <SwitcherInput items={items} selectedId={value} />;
};

export default ObservabilityMetricViewSwitcher;
