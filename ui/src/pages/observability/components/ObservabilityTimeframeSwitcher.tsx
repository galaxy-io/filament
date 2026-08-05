import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";

import { ObservabilityTimeframe } from "@/pages/observability/types";

interface ObservabilityTimeframeSwitcherProps {
  value: ObservabilityTimeframe;
  onChange: (timeframe: ObservabilityTimeframe) => void;
}

const ObservabilityTimeframeSwitcher = ({
  value,
  onChange,
}: ObservabilityTimeframeSwitcherProps) => {
  const items: SwitcherInputItem[] = Object.values(ObservabilityTimeframe).map((timeframe) => ({
    id: timeframe,
    label: timeframe,
    onClick: () => onChange(timeframe),
  }));

  return <SwitcherInput items={items} selectedId={value} />;
};

export default ObservabilityTimeframeSwitcher;
