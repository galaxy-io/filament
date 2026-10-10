import type { FC } from "react";

import NumberInput from "@galaxy-io/dls/inputs/NumberInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldInt: FC<FieldComponentProps> = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}) => (
  <NumberInput
    label={label}
    labelTooltip={field.help || undefined}
    isRequired={field.required}
    error={error}
    value={value === null || value === undefined ? null : Number(value)}
    onChange={(v) => onChange(v)}
    variant={variant}
    placeholder={`Enter ${label}...`}
    isDisabled={isDisabled}
    step={1}
    fillWidth
  />
);

export default FieldInt;
