import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import Field from "@galaxy-io/dls/inputs/Field";
import Box from "@galaxy-io/dls/layout/Box";
import Widget from "@galaxy-io/dls/widget/Widget";

import { INPUT_VARIANT_TO_CHECKBOX_INPUT_VARIANT_MAP } from "@/components/fields/constants";
import type { FieldComponentProps } from "@/components/fields/types";

const FieldBoolean = ({
  field,
  value,
  onChange,
  variant,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => (
  <Field label={label} isRequired={field.required} error={error} fillWidth>
    <Box fillWidth>
      <Widget>
        <CheckboxInput
          label={field.help}
          isChecked={(value as boolean) ?? false}
          onChange={(v) => onChange(v)}
          variant={variant && INPUT_VARIANT_TO_CHECKBOX_INPUT_VARIANT_MAP[variant]}
          isDisabled={isDisabled}
        />
      </Widget>
    </Box>
  </Field>
);

export default FieldBoolean;
