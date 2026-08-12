import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import Widget from "@galaxy-io/dls/widget/Widget";

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

  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required} error={error}>
      <Widget fillWidth>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
          {field.enum.map((option) => {
            const isChecked = selected.includes(option.value);
            return (
              <CheckboxInput
                key={option.value}
                label={option.label || option.value}
                isChecked={isChecked}
                onChange={(checked) => {
                  const next = new Set(selected);
                  if (checked) next.add(option.value);
                  else next.delete(option.value);
                  onChange(field.enum.map((item) => item.value).filter((value) => next.has(value)));
                }}
                isDisabled={isDisabled}
              />
            );
          })}
        </FlexWrapper>
      </Widget>
    </FieldWrapper>
  );
};

export default FieldList;
