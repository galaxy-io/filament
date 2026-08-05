import Switcher, { type SwitcherItem } from "@galaxy-io/dls/switcher/Switcher";

import { ObservabilityTimeframe } from "@/pages/observability/types";

interface ObservabilityTimeframeSwitcherProps {
  value: ObservabilityTimeframe;
  onChange: (timeframe: ObservabilityTimeframe) => void;
}

const ObservabilityTimeframeSwitcher = ({
  value,
  onChange,
}: ObservabilityTimeframeSwitcherProps) => {
  const items: SwitcherItem[] = Object.values(ObservabilityTimeframe).map((timeframe) => ({
    id: timeframe,
    label: timeframe,
    onClick: () => onChange(timeframe),
  }));

  return <Switcher items={items} selectedId={value} />;
};

export default ObservabilityTimeframeSwitcher;
