import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

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
  const selectedOption =
    METRIC_DIMENSION_PIVOT_OPTIONS.find((option) => option.value === value) ?? null;

  const handleChange = (option: SelectInputOption) => {
    onChange(option.value as MetricDimension);
  };

  const handleReset = () => {
    onChange(undefined);
  };

  return (
    <SelectInput
      options={METRIC_DIMENSION_PIVOT_OPTIONS}
      value={selectedOption}
      variant={InputVariant.TERTIARY}
      onChange={handleChange}
      onReset={handleReset}
      placeholder="Pivot"
      width={OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH}
    />
  );
};

export default ObservabilityPivotSelect;
