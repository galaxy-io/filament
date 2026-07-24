import { useMemo } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import { FIELD_TYPE_TO_FIELD_COMPONENT_MAP } from "@/pages/connectors/components/create/configure/fields/constants";
import Field from "@/pages/connectors/components/create/configure/fields/Field";
import { isFieldVisible } from "@/pages/connectors/components/create/configure/fields/visibility";
import { formatFieldName } from "@/pages/connectors/utils";

import { type ConfigField, FieldType } from "@/gen/ingestion/v1/common_pb";

interface CreateConnectionFieldProps {
  field: ConfigField;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  error?: string;
  isDisabled?: boolean;
  path?: string;
  getError?: (path: string) => string | undefined;
}

function asJsonObject(value: JsonValue): Record<string, JsonValue> {
  return value !== null && typeof value === "object" && !Array.isArray(value) ? value : {};
}

const CreateConnectionField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  path = field.name,
  getError,
}: CreateConnectionFieldProps) => {
  const label = useMemo(() => formatFieldName(field.name), [field.name]);

  if (field.type === FieldType.OBJECT && field.fields.length > 0) {
    const objectValue = asJsonObject(value);
    return (
      <Field
        label={label}
        help={field.help}
        isRequired={field.required}
        error={getError?.(path) ?? error}
        isSection
      >
        <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
          {field.fields
            .filter((child) => isFieldVisible(child, objectValue))
            .map((child) => {
              const childPath = `${path}.${child.name}`;
              return (
                <CreateConnectionField
                  key={child.name}
                  field={child}
                  value={objectValue[child.name] ?? null}
                  onChange={(childValue) =>
                    onChange({
                      ...objectValue,
                      [child.name]: childValue,
                    })
                  }
                  error={getError?.(childPath)}
                  isDisabled={isDisabled}
                  path={childPath}
                  getError={getError}
                />
              );
            })}
        </FlexWrapper>
      </Field>
    );
  }

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
