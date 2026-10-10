import type { FC } from "react";

import ToggleInput, {
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";

import { ObservabilityTimeframe } from "@/pages/observability/types";

interface ObservabilityTimeframeSwitcherProps {
  value: ObservabilityTimeframe;
  onChange: (timeframe: ObservabilityTimeframe) => void;
}

const ObservabilityTimeframeSwitcher: FC<ObservabilityTimeframeSwitcherProps> = ({
  value,
  onChange,
}) => {
  const items: ToggleOption<ObservabilityTimeframe>[] = Object.values(ObservabilityTimeframe).map(
    (timeframe) => ({ id: timeframe, label: timeframe }),
  );

  return (
    <ToggleInput
      variant={ToggleInputVariant.PRIMARY}
      options={items}
      value={value}
      onChange={onChange}
    />
  );
};

export default ObservabilityTimeframeSwitcher;
