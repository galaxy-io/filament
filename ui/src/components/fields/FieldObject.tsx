import { type FC, useState } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
import Field from "@galaxy-io/dls/inputs/Field";

import type { FieldComponentProps } from "@/components/fields/types";

interface FieldObjectState {
  displayValue: string;
  emittedValue: string;
  parseError?: string;
}

const FieldObject: FC<FieldComponentProps> = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}) => {
  const serializedValue = value ? JSON.stringify(value, null, 2) : "";
  const [state, setState] = useState<FieldObjectState>(() => ({
    displayValue: serializedValue,
    emittedValue: serializedValue,
  }));
  const isExternalChange = serializedValue !== state.emittedValue;
  const displayValue = isExternalChange ? serializedValue : state.displayValue;
  const parseError = isExternalChange ? undefined : state.parseError;

  const handleChange = (next: string) => {
    if (next.trim() === "") {
      setState({ displayValue: next, emittedValue: "" });
      onChange(null);
      return;
    }
    try {
      const parsed: JsonValue = JSON.parse(next);
      if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
        setState({
          displayValue: next,
          emittedValue: serializedValue,
          parseError: "Enter a JSON object",
        });
        return;
      }
      setState({ displayValue: next, emittedValue: JSON.stringify(parsed, null, 2) });
      onChange(parsed);
    } catch {
      setState({
        displayValue: next,
        emittedValue: serializedValue,
        parseError: "Enter valid JSON",
      });
    }
  };

  return (
    <Field
      label={label}
      labelTooltip={field.help || undefined}
      isRequired={field.required}
      error={parseError ?? error}
      fillWidth
    >
      <CodeEditor
        value={displayValue}
        onChange={handleChange}
        placeholder="{}"
        language={CodeEditorLanguage.JSON}
        isReadOnly={isDisabled}
        hasLineNumbers={false}
      />
    </Field>
  );
};

export default FieldObject;
