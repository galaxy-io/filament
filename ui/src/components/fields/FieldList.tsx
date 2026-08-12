import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponentProps } from "@/components/fields/types";

const FieldList = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const selected = Array.isArray(value) ? value : [];
  const options: SelectInputOption[] = field.enum.map((option) => ({
    id: option.value,
    label: option.label || option.value,
    value: option.value,
  }));
  const selectedOptions = options.filter((option) => selected.includes(option.value as string));
  const handleChange = (next: SelectInputOption[]) => {
    onChange(next.map((option) => option.value as string));
  };
  const handleReset = () => {
    onChange([]);
  };

  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required}>
      <MultiSelectInput
        options={options}
        value={selectedOptions}
        onChange={handleChange}
        onReset={handleReset}
        size={InputSize.LARGE}
        placeholder={`Select ${label}...`}
        error={error}
        isDisabled={isDisabled}
        fillWidth
      />
    </FieldWrapper>
  );
};

export default FieldList;
