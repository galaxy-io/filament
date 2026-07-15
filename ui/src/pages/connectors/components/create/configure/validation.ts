import { create, type JsonValue } from "@bufbuild/protobuf";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import { type ValidationError, ValidationErrorSchema } from "@/gen/ingestion/v1/providers_pb";

export function isEmptyJsonValue(value: JsonValue): boolean {
  return (
    value === undefined || value === null || (typeof value === "string" && value.trim() === "")
  );
}

export function validateRequiredFields(
  fields: ConfigField[],
  config: Record<string, JsonValue>,
): ValidationError[] {
  const errors: ValidationError[] = [];

  for (const field of fields) {
    if (!field.required) continue;

    const value = config[field.name];

    if (isEmptyJsonValue(value)) {
      errors.push(
        create(ValidationErrorSchema, {
          field: field.name,
          message: `${field.name} is required`,
        }),
      );
    }
  }

  return errors;
}

export function createRequiredFieldsValidationErrorMap(
  errors: ValidationError[],
): Map<string, string> {
  return new Map(errors.map((e) => [e.field, e.message]));
}

export function isNameValid(name: string | undefined): boolean {
  return (name ?? "").trim().length > 0;
}

export function getNameError(name: string | undefined, shouldShowErrors: boolean): string | null {
  return shouldShowErrors && !isNameValid(name) ? "Name is required" : null;
}
