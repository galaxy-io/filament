import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";

import Field from "@/pages/connectors/components/create/configure/fields/Field";
import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

const FieldBoolean = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <Field label={label} help={field.help} isRequired={field.required} error={error}>
      <ToggleInput
        value={(value as boolean) ?? false}
        onChange={(v) => onChange(v)}
        isDisabled={isDisabled}
      />
    </Field>
  );
};

export default FieldBoolean;
