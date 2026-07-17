import NumberInput from "@galaxy-io/dls/inputs/NumberInput";

import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

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
      onChange={(v) => onChange(v)}
      placeholder={`Enter ${label}...`}
      label={label}
      labelTooltip={field.help}
      isRequired={field.required}
      error={error}
      isDisabled={isDisabled}
      step={1}
      fillWidth
    />
  );
};

export default FieldInt;
