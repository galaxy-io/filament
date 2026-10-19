import type { FC } from "react";

import SelectInput, { type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import {
  OBSERVABILITY_REFRESH_INTERVAL_SELECT_WIDTH,
  OBSERVABILITY_REFRESH_INTERVAL_TO_LABEL_MAP,
} from "@/pages/observability/constants";
import { ObservabilityRefreshInterval } from "@/pages/observability/types";

interface ObservabilityRefreshIntervalSelectProps {
  value: ObservabilityRefreshInterval;
  onChange: (refreshInterval: ObservabilityRefreshInterval) => void;
}

const ObservabilityRefreshIntervalSelect: FC<ObservabilityRefreshIntervalSelectProps> = ({
  value,
  onChange,
}) => {
  const options: SelectOption<ObservabilityRefreshInterval>[] = Object.values(
    ObservabilityRefreshInterval,
  ).map((refreshInterval) => ({
    id: refreshInterval,
    label: OBSERVABILITY_REFRESH_INTERVAL_TO_LABEL_MAP[refreshInterval],
  }));

  return (
    <Box width={OBSERVABILITY_REFRESH_INTERVAL_SELECT_WIDTH}>
      <SelectInput
        fillWidth
        options={options}
        value={value}
        onChange={(refreshInterval) => {
          if (refreshInterval !== null) onChange(refreshInterval);
        }}
        ariaLabel="Refresh interval"
      />
    </Box>
  );
};

export default ObservabilityRefreshIntervalSelect;
