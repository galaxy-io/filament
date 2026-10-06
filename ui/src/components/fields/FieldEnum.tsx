import SelectInput, { type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import { INPUT_VARIANT_TO_SELECT_INPUT_VARIANT_MAP } from "@/components/fields/constants";
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
    <SelectInput
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={error}
      options={options}
      value={typeof value === "string" ? value : null}
      onChange={(id) => onChange(id)}
      variant={variant && INPUT_VARIANT_TO_SELECT_INPUT_VARIANT_MAP[variant]}
      placeholder={`Select ${label}...`}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default FieldEnum;
