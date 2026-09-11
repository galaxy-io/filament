import type { ClipboardEvent } from "react";

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

  const handlePaste = (event: ClipboardEvent<HTMLDivElement>) => {
    const pasted = event.clipboardData.getData("text/plain");
    if (!pasted.includes("\n") && !pasted.includes("\r")) return;

    // A single-line input strips line breaks during its paste.
    // Store multiline secrets directly in form state while retaining the
    // standard masked password control.
    event.preventDefault();
    onChange(pasted);
  };

  return (
    <div onPasteCapture={handlePaste} style={{ width: "100%" }}>
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
    </div>
  );
};

export default FieldSecret;
