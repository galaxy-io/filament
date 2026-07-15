import { create, type JsonValue } from "@bufbuild/protobuf";

import { FieldType, type ConfigField } from "@/gen/ingestion/v1/common_pb";
import {
  ValidationErrorSchema,
  type ValidationError,
} from "@/gen/ingestion/v1/providers_pb";

export function validateRequiredFields(
  fields: ConfigField[],
  config: Record<string, JsonValue>,
  secretValues: Record<string, string>,
): ValidationError[] {
  const errors: ValidationError[] = [];

  for (const field of fields) {
    if (!field.required) continue;

    if (field.type === FieldType.SECRET) {
      const value = secretValues[field.name];
      if (!value || value.trim() === "") {
        errors.push(
          create(ValidationErrorSchema, {
            field: field.name,
            message: `${field.name} is required`,
          }),
        );
      }
    } else {
      const value = config[field.name];
      if (value === undefined || value === null || value === "") {
        errors.push(
          create(ValidationErrorSchema, {
            field: field.name,
            message: `${field.name} is required`,
          }),
        );
      }
    }
  }

  return errors;
}

export function createRequiredFieldsValidationErrorMap(
  errors: ValidationError[],
): Map<string, string> {
  return new Map(errors.map((e) => [e.field, e.message]));
}
