import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import type { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import {
  METRIC_DIMENSION_PIVOT_OPTIONS,
  OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH,
} from "@/pages/observability/components/timeseries/constants";

interface ObservabilityPivotSelectProps {
  value: MetricDimension | undefined;
  onChange: (pivot: MetricDimension | undefined) => void;
}

const ObservabilityPivotSelect = ({ value, onChange }: ObservabilityPivotSelectProps) => {
  const handleChange = (id: string | null) => {
    onChange(id === null ? undefined : (Number(id) as MetricDimension));
  };

  return (
    <Box width={OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH}>
      <SelectInput
        fillWidth
        options={METRIC_DIMENSION_PIVOT_OPTIONS}
        value={value === undefined ? null : String(value)}
        variant={SelectInputVariant.PRIMARY}
        onChange={handleChange}
        isClearable
        placeholder="Pivot"
      />
    </Box>
  );
};

export default ObservabilityPivotSelect;
