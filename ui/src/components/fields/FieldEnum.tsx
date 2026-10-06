import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import { INPUT_VARIANT_TO_SELECT_INPUT_VARIANT_MAP } from "@/components/fields/constants";
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
  }));

  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required}>
      <SelectInput
        options={options}
        value={typeof value === "string" ? value : null}
        onChange={(id) => onChange(id)}
        variant={variant && INPUT_VARIANT_TO_SELECT_INPUT_VARIANT_MAP[variant]}
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
