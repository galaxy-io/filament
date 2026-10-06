import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponentProps } from "@/components/fields/types";

const FieldEnum = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const options: SelectOption[] = field.enum.map((enumOption) => ({
    id: enumOption.value,
    label: enumOption.label || enumOption.value,
    value: enumOption.value,
  }));

  const selectedOption = options.find((opt) => opt.value === value) ?? null;

  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required}>
      <SelectInput
        options={options}
        /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
          selectedOption
        }
        onChange={(opt) => onChange(opt.value as string)}
        /* @dls-migrate selectinput.enums: Pass a `SelectInputSize` / `SelectInputVariant` member (same names as `InputSize` / `InputVariant`). */ variant={
          variant
        }
        placeholder={`Select ${label}...`}
        error={error}
        isDisabled={isDisabled}
        size={SelectInputSize.LARGE}
        fillWidth
      />
    </FieldWrapper>
  );
};

export default FieldEnum;
