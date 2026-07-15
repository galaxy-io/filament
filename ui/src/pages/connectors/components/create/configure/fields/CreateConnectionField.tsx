import { useMemo } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import { FIELD_TYPE_TO_FIELD_COMPONENT_MAP } from "@/pages/connectors/components/create/configure/fields/constants";
import { formatFieldName } from "@/pages/connectors/utils";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

interface CreateConnectionFieldProps {
  field: ConfigField;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  error?: string;
  isDisabled?: boolean;
}

const CreateConnectionField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
}: CreateConnectionFieldProps) => {
  const label = useMemo(() => formatFieldName(field.name), [field.name]);

  const Component = FIELD_TYPE_TO_FIELD_COMPONENT_MAP[field.type];
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

export default CreateConnectionField;
