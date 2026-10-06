import Field from "@galaxy-io/dls/inputs/Field";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldString = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <Field
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={error}
      fillWidth
    >
      <TextInput
        value={(value as string) ?? ""}
        onChange={(v) => onChange(v)}
        variant={variant}
        placeholder={`Enter ${label}...`}
        isDisabled={isDisabled}
        fillWidth
      />
    </Field>
  );
};

export default FieldString;
