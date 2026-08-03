import { InputSize } from "@galaxy-io/dls/inputs/Input";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldSecret = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
  secretPlaceholder,
}: FieldComponentProps) => {
  return (
    <PasswordInput
      value={(value as string) ?? ""}
      onChange={(v) => onChange(v)}
      size={InputSize.LARGE}
      placeholder={secretPlaceholder ?? `Enter ${label}...`}
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
