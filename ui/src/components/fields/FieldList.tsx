import type { FC } from "react";

import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import TagInput from "@galaxy-io/dls/inputs/TagInput";

import {
  INPUT_VARIANT_TO_MULTI_SELECT_INPUT_VARIANT_MAP,
  INPUT_VARIANT_TO_TAG_INPUT_VARIANT_MAP,
} from "@/components/fields/constants";
import type { FieldComponentProps } from "@/components/fields/types";

import {
  getSelectAllChange,
  getSelectAllOptions,
  getSelectAllValue,
  type SelectAllOption,
} from "@/utils/select";

const FieldList: FC<FieldComponentProps> = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}) => {
  const selected = Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string")
    : [];
  const options: SelectOption[] = field.enum.map((option) => ({
    id: option.value,
    label: option.label || option.value,
  }));
  const selectAll: SelectAllOption = {
    id: "select-all",
    label: `All ${label.toLowerCase()}`,
    optionIds: options.map((option) => option.id),
  };
  const handleChange = (next: string[]) => {
    onChange(getSelectAllChange(selectAll, next, selected));
  };

  if (field.enum.length === 0) {
    return (
      <TagInput
        label={label}
        labelTooltip={field.help || undefined}
        isRequired={field.required}
        error={error}
        value={selected}
        onChange={onChange}
        variant={variant && INPUT_VARIANT_TO_TAG_INPUT_VARIANT_MAP[variant]}
        placeholder="Press Enter or comma to add a value"
        isDisabled={isDisabled}
        fillWidth
      />
    );
  }

  return (
    <MultiSelectInput
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={error}
      options={getSelectAllOptions(selectAll, options)}
      pinnedIds={[selectAll.id]}
      value={getSelectAllValue(selectAll, selected)}
      onChange={handleChange}
      isClearable
      variant={variant && INPUT_VARIANT_TO_MULTI_SELECT_INPUT_VARIANT_MAP[variant]}
      placeholder={`Select ${label}...`}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default FieldList;
