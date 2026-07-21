import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

const FieldEnum = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const options: SelectInputOption[] = field.enum.map((enumValue) => ({
    id: enumValue,
    label: enumValue,
    value: enumValue,
  }));

  const selectedOption = options.find((opt) => opt.value === value) ?? null;

  return (
    <SelectInput
      options={options}
      value={selectedOption}
      onChange={(opt) => onChange(opt.value as string)}
      placeholder={`Select ${label}...`}
      label={label}
      isRequired={field.required}
      error={error}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default FieldEnum;
