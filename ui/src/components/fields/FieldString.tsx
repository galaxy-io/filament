import type { FC } from "react";

import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldString: FC<FieldComponentProps> = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}) => (
  <TextInput
    label={label}
    labelTooltip={field.help || undefined}
    isRequired={field.required}
    error={error}
    value={(value as string) ?? ""}
    onChange={(v) => onChange(v)}
    variant={variant}
    placeholder={`Enter ${label}...`}
    isDisabled={isDisabled}
    fillWidth
  />
);

export default FieldString;
