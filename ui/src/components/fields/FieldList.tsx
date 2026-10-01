import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import MultiTextInput from "@galaxy-io/dls/inputs/MultiTextInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponentProps } from "@/components/fields/types";

const FieldList = ({
  field,
  value,
  onChange,
  variant,
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

  if (field.name === "brokers" && field.enum.length === 0) {
    return (
      <FieldWrapper
        label={label}
        help="Kafka bootstrap broker addresses (host:port). Press Enter or comma to add an address."
        isRequired={field.required}
      >
        <MultiTextInput
          value={selected.filter((address): address is string => typeof address === "string")}
          onChange={onChange}
          size={InputSize.LARGE}
          variant={variant}
          placeholder="localhost:9092"
          error={error}
          isDisabled={isDisabled}
          fillWidth
        />
      </FieldWrapper>
    );
  }

  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required}>
      <MultiSelectInput
        options={options}
        pinnedOptions={[
          {
            id: "select-all",
            label: `All ${label.toLowerCase()}`,
            optionIds: options.map((option) => option.value as string),
          },
        ]}
        value={selectedOptions}
        onChange={handleChange}
        onReset={handleReset}
        size={InputSize.LARGE}
        variant={variant}
        placeholder={`Select ${label}...`}
        error={error}
        isDisabled={isDisabled}
        fillWidth
      />
    </FieldWrapper>
  );
};

export default FieldList;
