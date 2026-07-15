import { useMemo } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import { formatFieldName } from "@/pages/connectors/utils";

import { FIELD_COMPONENTS } from "./fields/constants";

interface DynamicFieldProps {
  field: ConfigField;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  error?: string;
  isDisabled?: boolean;
}

const DynamicField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
}: DynamicFieldProps) => {
  const label = useMemo(() => formatFieldName(field.name), [field.name]);

  const Component = FIELD_COMPONENTS[field.type];
  if (!Component) return null;

  return (
    <Component
      field={field}
      value={value}
      onChange={onChange}
      error={error}
      isDisabled={isDisabled}
      label={label}
    />
  );
};

export default DynamicField;
