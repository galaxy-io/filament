import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponentProps } from "@/components/fields/types";

const selectedValues = (value: FieldComponentProps["value"]): string[] => {
  if (Array.isArray(value)) return value.filter((item): item is string => typeof item === "string");

  // Existing connections may still contain the comma-separated string used
  // before list fields were introduced.
  if (typeof value === "string") {
    return value
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }

  return [];
};

const FieldList = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const selected = selectedValues(value);
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
