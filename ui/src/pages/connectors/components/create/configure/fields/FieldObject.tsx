import { useEffect, useRef, useState } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";

import Field from "@/pages/connectors/components/create/configure/fields/Field";
import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

interface FieldObjectState {
  displayValue: string;
  parseError?: string;
}

const DEFAULT_STATE: FieldObjectState = {
  displayValue: "",
};

const FieldObject = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const serializedValue = value ? JSON.stringify(value, null, 2) : "";
  const [state, setState] = useState<FieldObjectState>(() => ({
    ...DEFAULT_STATE,
    displayValue: serializedValue,
  }));
  const lastEmitted = useRef(serializedValue);

  useEffect(() => {
    if (serializedValue !== lastEmitted.current) {
      lastEmitted.current = serializedValue;
      setState((prev) => ({ ...prev, displayValue: serializedValue }));
    }
  }, [serializedValue]);

  const handleChange = (next: string) => {
    if (next.trim() === "") {
      setState((prev) => ({ ...prev, displayValue: next, parseError: undefined }));
      lastEmitted.current = "";
      onChange(null);
      return;
    }
    try {
      const parsed: JsonValue = JSON.parse(next);
      if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
        setState((prev) => ({ ...prev, displayValue: next, parseError: "Enter a JSON object" }));
        return;
      }
      setState((prev) => ({ ...prev, displayValue: next, parseError: undefined }));
      lastEmitted.current = JSON.stringify(parsed, null, 2);
      onChange(parsed);
    } catch {
      setState((prev) => ({ ...prev, displayValue: next, parseError: "Enter valid JSON" }));
    }
  };

  return (
    <Field
      label={label}
      help={field.help}
      isRequired={field.required}
      error={state.parseError ?? error}
    >
      <CodeEditor
        content={state.displayValue}
        onChange={handleChange}
        placeholder="{}"
        lang="json"
        isReadOnly={isDisabled}
        noLineNumbers
      />
    </Field>
  );
};

export default FieldObject;
