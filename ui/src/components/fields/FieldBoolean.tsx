import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import Widget from "@galaxy-io/dls/widget/Widget";

import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponentProps } from "@/components/fields/types";

const FieldBoolean = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <FieldWrapper label={label} help={field.help} isRequired={field.required} error={error}>
      <Widget fillWidth>
        <CheckboxInput
          label={field.help}
          isChecked={(value as boolean) ?? false}
          onChange={(v) => onChange(v)}
          variant={variant}
          isDisabled={isDisabled}
        />
      </Widget>
    </FieldWrapper>
  );
};

export default FieldBoolean;
