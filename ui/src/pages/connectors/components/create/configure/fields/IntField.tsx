import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { FieldComponentProps } from "./types";

const IntField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <TextInput
      value={value !== null && value !== undefined ? String(value) : ""}
      onChange={(v) => {
        if (v === "" || /^-?\d*$/.test(v)) {
          onChange(v);
        }
      }}
      placeholder={`Enter ${label}...`}
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={error}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default IntField;
