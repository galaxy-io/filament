import type { FC } from "react";

import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import {
  OBSERVABILITY_TIMESERIES_PIVOT_OPTIONS,
  OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH,
} from "@/pages/observability/components/timeseries/constants";

import { mapOptionIdToEnum } from "@/utils/select";

interface ObservabilityPivotSelectProps {
  value: MetricDimension | undefined;
  onChange: (pivot: MetricDimension | undefined) => void;
}

const ObservabilityPivotSelect: FC<ObservabilityPivotSelectProps> = ({ value, onChange }) => {
  const handleChange = (id: string | null) => {
    onChange(id === null ? undefined : mapOptionIdToEnum(MetricDimension, id));
  };

  return (
    <Box width={OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH}>
      <SelectInput
        fillWidth
        options={OBSERVABILITY_TIMESERIES_PIVOT_OPTIONS}
        value={value === undefined ? null : String(value)}
        size={SelectInputSize.SMALL}
        variant={SelectInputVariant.PRIMARY}
        onChange={handleChange}
        isClearable
        placeholder="Pivot"
      />
    </Box>
  );
};

export default ObservabilityPivotSelect;
