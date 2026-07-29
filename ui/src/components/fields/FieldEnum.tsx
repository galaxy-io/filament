import SelectInput, {
  type SelectInputOption,
  SelectInputSize,
} from "@galaxy-io/dls/inputs/SelectInput";

import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponentProps } from "@/components/fields/types";

const FieldEnum = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const options: SelectInputOption[] = field.enum.map((enumOption) => ({
    id: enumOption.value,
    label: enumOption.label || enumOption.value,
    value: enumOption.value,
  }));

  const selectedOption = options.find((opt) => opt.value === value) ?? null;

  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required}>
      <SelectInput
        options={options}
        value={selectedOption}
        onChange={(opt) => onChange(opt.value as string)}
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
