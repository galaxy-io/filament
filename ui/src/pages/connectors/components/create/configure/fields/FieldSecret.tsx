import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

const FieldSecret = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <PasswordInput
      value={(value as string) ?? ""}
      onChange={(v) => onChange(v)}
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

export default FieldSecret;
