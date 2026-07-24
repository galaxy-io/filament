import { create, type JsonValue } from "@bufbuild/protobuf";

import { isFieldVisible } from "@/pages/connectors/components/create/configure/fields/visibility";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import type { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";
import { type ValidationError, ValidationErrorSchema } from "@/gen/ingestion/v1/providers_pb";

export function isEmptyJsonValue(value: JsonValue): boolean {
  return (
    value === undefined || value === null || (typeof value === "string" && value.trim() === "")
  );
}

function isJsonObject(value: JsonValue): value is Record<string, JsonValue> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function validateRequiredFields(
  fields: ConfigField[],
  config: Record<string, JsonValue>,
): ValidationError[] {
  const errors: ValidationError[] = [];

  const visit = (field: ConfigField, config: Record<string, JsonValue>, path: string) => {
    if (!isFieldVisible(field, config)) return;

    const value = config[field.name];
    const empty = isEmptyJsonValue(value);
    if (field.required && empty) {
      errors.push(
        create(ValidationErrorSchema, {
          field: path,
          message: `${field.name} is required`,
        }),
      );
    }
    if (!empty && field.fields.length > 0 && isJsonObject(value)) {
      for (const child of field.fields) {
        visit(child, value, `${path}.${child.name}`);
      }
    }
  };

  for (const field of fields) visit(field, config, field.name);

  return errors;
}

export function createRequiredFieldsValidationErrorMap(
  errors: ValidationError[],
): Map<string, string> {
  return new Map(errors.map((e) => [e.field, e.message]));
}

export function isNameValid(name: CreateConnectionRequest["name"] | undefined): boolean {
  return (name ?? "").trim().length > 0;
}

export function getNameError(
  name: CreateConnectionRequest["name"] | undefined,
  shouldShowErrors: boolean,
): string | null {
  return shouldShowErrors && !isNameValid(name) ? "Name is required" : null;
}
