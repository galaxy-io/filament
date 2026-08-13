import { InputSize } from "@galaxy-io/dls/inputs/Input";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldSecret = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
  hasStoredSecret = false,
}: FieldComponentProps) => {
  const placeholder = hasStoredSecret ? "Leave blank to keep current value" : `Enter ${label}...`;

  return (
    <PasswordInput
      value={(value as string) ?? ""}
      onChange={(v) => onChange(v)}
      size={InputSize.LARGE}
      variant={variant}
      placeholder={placeholder}
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
