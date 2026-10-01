import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

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
      <TextInput
        value={typeof selected[0] === "string" ? selected[0] : ""}
        onChange={(address) => onChange(address ? [address] : [])}
        size={InputSize.LARGE}
        variant={variant}
        label="Instance address"
        placeholder="localhost:9092"
        labelTooltip="Kafka bootstrap instance address (host:port)."
        isRequired={field.required}
        error={error}
        isDisabled={isDisabled}
        fillWidth
      />
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
