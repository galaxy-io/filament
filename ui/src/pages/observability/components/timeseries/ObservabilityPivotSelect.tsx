import SelectInput, {
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
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
  const selectedOption =
    METRIC_DIMENSION_PIVOT_OPTIONS.find((option) => option.value === value) ?? null;

  const handleChange = (option: SelectOption) => {
    onChange(option.value as MetricDimension);
  };

  const handleReset = () => {
    onChange(undefined);
  };

  return (
    <Box width={OBSERVABILITY_TIMESERIES_PIVOT_SELECT_WIDTH}>
      <SelectInput
        fillWidth
        options={METRIC_DIMENSION_PIVOT_OPTIONS}
        /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
          selectedOption
        }
        variant={SelectInputVariant.TERTIARY}
        onChange={handleChange}
        /* @dls-migrate selectinput.onReset: The clear button calls `onChange` with an empty value: move side effects there and add `isClearable`. */ onReset={
          handleReset
        }
        placeholder="Pivot"
      />
    </Box>
  );
};

export default ObservabilityPivotSelect;
