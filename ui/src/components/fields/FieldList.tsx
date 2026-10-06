import MultiSelectInput, { MultiSelectInputSize } from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import TagInput, { TagInputSize } from "@galaxy-io/dls/inputs/TagInput";

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
  const options: SelectOption[] = field.enum.map((option) => ({
    id: option.value,
    label: option.label || option.value,
    value: option.value,
  }));
  const selectedOptions = options.filter((option) => selected.includes(option.value as string));
  const handleChange = (next: SelectOption[]) => {
    onChange(next.map((option) => option.value as string));
  };
  const handleReset = () => {
    onChange([]);
  };

  // No enum means there is nothing to select from; the user types the values.
  if (field.enum.length === 0) {
    return (
      <FieldWrapper label={label} help={field.help} isRequired={field.required}>
        <TagInput
          value={selected.filter((item): item is string => typeof item === "string")}
          onChange={onChange}
          size={TagInputSize.LARGE}
          /* @dls-migrate taginput.enums: Pass a `TagInputSize` / `TagInputVariant` member (same names as `InputSize` / `InputVariant`). */ variant={
            variant
          }
          placeholder="Press Enter or comma to add a value"
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
        /* @dls-migrate multiselectinput.pinnedOptions: Pinned rows are now option ids: pass `pinnedIds`. */ pinnedOptions={[
          {
            id: "select-all",
            label: `All ${label.toLowerCase()}`,
            optionIds: options.map((option) => option.value as string),
          },
        ]}
        /* @dls-migrate multiselectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
          selectedOptions
        }
        onChange={handleChange}
        /* @dls-migrate multiselectinput.onReset: The clear button calls `onChange` with an empty value: move side effects there and add `isClearable`. */ onReset={
          handleReset
        }
        size={MultiSelectInputSize.LARGE}
        /* @dls-migrate multiselectinput.enums: Pass a `MultiSelectInputSize` / `MultiSelectInputVariant` member (same names as `InputSize` / `InputVariant`). */ variant={
          variant
        }
        placeholder={`Select ${label}...`}
        error={error}
        isDisabled={isDisabled}
        fillWidth
      />
    </FieldWrapper>
  );
};

export default FieldList;
