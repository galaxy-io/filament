import type { ClipboardEvent, FC } from "react";

import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldSecret: FC<FieldComponentProps> = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
  hasStoredSecret = false,
}) => {
  const placeholder = hasStoredSecret ? "Leave blank to keep current value" : `Enter ${label}...`;

  const handlePaste = (event: ClipboardEvent<HTMLInputElement>) => {
    const pasted = event.clipboardData.getData("text/plain");
    if (!pasted.includes("\n") && !pasted.includes("\r")) return;
    event.preventDefault();
    onChange(pasted);
  };

  return (
    <PasswordInput
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={error}
      value={(value as string) ?? ""}
      onChange={(v) => onChange(v)}
      onPaste={handlePaste}
      variant={variant}
      placeholder={placeholder}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default FieldSecret;
