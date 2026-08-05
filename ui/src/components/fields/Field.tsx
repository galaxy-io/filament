import { useMemo } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import { type ConfigField, FieldType } from "@/gen/ingestion/v1/common_pb";

import FieldBoolean from "@/components/fields/FieldBoolean";
import FieldEnum from "@/components/fields/FieldEnum";
import FieldInt from "@/components/fields/FieldInt";
import FieldObject from "@/components/fields/FieldObject";
import FieldSecret from "@/components/fields/FieldSecret";
import FieldString from "@/components/fields/FieldString";
import FieldWrapper from "@/components/fields/FieldWrapper";
import type { FieldComponent } from "@/components/fields/types";
import { formatFieldName, isFieldVisible, isJsonObject } from "@/components/fields/utils";

const FIELD_TYPE_TO_FIELD_COMPONENT_MAP: Partial<Record<FieldType, FieldComponent>> = {
  [FieldType.STRING]: FieldString,
  [FieldType.UNSPECIFIED]: FieldString,
  [FieldType.DURATION]: FieldString,
  [FieldType.INT]: FieldInt,
  [FieldType.BOOL]: FieldBoolean,
  [FieldType.SECRET]: FieldSecret,
  [FieldType.ENUM]: FieldEnum,
  [FieldType.OBJECT]: FieldObject,
};

interface FieldProps {
  field: ConfigField;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  isDisabled?: boolean;
  path?: string;
  getError?: (path: string) => string | undefined;
  hasStoredSecret?: boolean;
}

const Field = ({
  field,
  value,
  onChange,
  isDisabled = false,
  path = field.name,
  getError,
  hasStoredSecret,
}: FieldProps) => {
  const label = useMemo(() => formatFieldName(field.name), [field.name]);

  if (field.type === FieldType.OBJECT && field.fields.length > 0) {
    const objectValue = isJsonObject(value) ? value : {};
    return (
      <FieldWrapper
        label={label}
        help={field.help}
        isRequired={field.required}
        error={getError?.(path)}
        isSection
      >
        <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
          {field.fields
            .filter((child) => isFieldVisible(child, objectValue))
            .map((child) => (
              <Field
                key={child.name}
                field={child}
                value={objectValue[child.name] ?? null}
                onChange={(childValue) =>
                  onChange({
                    ...objectValue,
                    [child.name]: childValue,
                  })
                }
                isDisabled={isDisabled}
                path={`${path}.${child.name}`}
                getError={getError}
                hasStoredSecret={hasStoredSecret}
              />
            ))}
        </FlexWrapper>
      </FieldWrapper>
    );
  }

  const Component = FIELD_TYPE_TO_FIELD_COMPONENT_MAP[field.type];
  if (!Component) return null;

  return (
    <Component
      field={field}
      value={value}
      onChange={onChange}
      error={getError?.(path)}
      isDisabled={isDisabled}
      label={label}
      hasStoredSecret={hasStoredSecret}
    />
  );
};

export default Field;
