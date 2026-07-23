import { useEffect, useState } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

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
  const serializedValue = value ? JSON.stringify(value, null, 2) : "";
  const [displayValue, setDisplayValue] = useState(serializedValue);
  const [parseError, setParseError] = useState<string>();

  useEffect(() => setDisplayValue(serializedValue), [serializedValue]);

  const handleChange = (next: string) => {
    setDisplayValue(next);
    if (next.trim() === "") {
      setParseError(undefined);
      onChange(null);
      return;
    }
    try {
      const parsed: JsonValue = JSON.parse(next);
      if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
        setParseError("Enter a JSON object");
        return;
      }
      setParseError(undefined);
      onChange(parsed);
    } catch {
      setParseError("Enter valid JSON");
    }
  };

  return (
    <Field label={label} help={field.help} isRequired={field.required} error={parseError ?? error}>
      <CodeEditor
        content={displayValue}
        onChange={handleChange}
        placeholder={field.help || "Enter JSON..."}
        lang="json"
        isReadOnly={isDisabled}
      />
    </Field>
  );
};

export default FieldObject;
