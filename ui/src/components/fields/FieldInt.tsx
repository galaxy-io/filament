import { InputSize } from "@galaxy-io/dls/inputs/Input";
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldInt = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <NumberInput
      value={value !== null && value !== undefined ? Number(value) : undefined}
      size={InputSize.LARGE}
      onChange={(v) => onChange(v)}
      placeholder={`Enter ${label}...`}
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={error}
      isDisabled={isDisabled}
      step={1}
      fillWidth
    />
  );
};

export default FieldInt;
