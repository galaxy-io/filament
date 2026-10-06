import Field from "@galaxy-io/dls/inputs/Field";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldInt = ({
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
      <NumberInput
        value={value !== null && value !== undefined ? Number(value) : undefined}
        size={InputSize.LARGE}
        /* @dls-migrate numberinput.onChange: `onChange` now runs on commit with `number | null`: check the handler. */ onChange={(
          v,
        ) => onChange(v)}
        variant={variant}
        placeholder={`Enter ${label}...`}
        isDisabled={isDisabled}
        step={1}
        fillWidth
      />
    </Field>
  );
};

export default FieldInt;
