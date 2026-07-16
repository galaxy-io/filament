import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";

import Field from "@/pages/connectors/components/create/configure/fields/Field";
import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

const FieldObject = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const displayValue =
    typeof value === "string" ? value : value ? JSON.stringify(value, null, 2) : "";

  return (
    <Field label={label} help={field.help} isRequired={field.required} error={error}>
      <CodeEditor
        content={displayValue}
        onChange={(v) => onChange(v)}
        placeholder={field.help || "Enter JSON..."}
        lang="json"
        isReadOnly={isDisabled}
      />
    </Field>
  );
};

export default FieldObject;
