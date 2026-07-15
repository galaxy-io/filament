import { toJson, type JsonValue } from "@bufbuild/protobuf";
import { ValueSchema, type Value } from "@bufbuild/protobuf/wkt";

import { FieldScope, FieldType, type ConfigField } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import type { CreateConnectionConfigureState } from "./types";

export function getConnectionScopedFields(connector: ConnectorSpec): ConfigField[] {
  return (
    connector.configSchema?.fields?.filter(
      (f) => f.scope === FieldScope.CONNECTION || f.scope === FieldScope.UNSPECIFIED,
    ) ?? []
  );
}

export function buildConfigObject(
  fields: ConfigField[],
  formValues: Record<string, JsonValue>,
  secretValues: Record<string, string>,
): Record<string, JsonValue> {
  const config: Record<string, JsonValue> = {};

  for (const field of fields) {
    if (field.type === FieldType.SECRET) {
      const secretValue = secretValues[field.name];
      if (secretValue !== undefined && secretValue !== "") {
        config[field.name] = secretValue;
      }
    } else {
      const value = formValues[field.name];
      if (value !== undefined && value !== null && value !== "") {
        if (field.type === FieldType.INT && typeof value === "string") {
          config[field.name] = Number.parseInt(value, 10);
        } else if (field.type === FieldType.OBJECT && typeof value === "string") {
          try {
            config[field.name] = JSON.parse(value);
          } catch {
            config[field.name] = value;
          }
        } else {
          config[field.name] = value;
        }
      }
    }
  }

  return config;
}

export function generateSecretRef(connectionName: string, fieldName: string): string {
  const timestamp = Date.now();
  const sanitizedName = connectionName.toLowerCase().replace(/[^a-z0-9-]/g, "-");
  return `${sanitizedName}-${fieldName}-${timestamp}`;
}

export function isConnectionNameProvided(state: CreateConnectionConfigureState): boolean {
  return state.connectionName.trim().length > 0;
}

export function getConnectionNameError(state: CreateConnectionConfigureState): string | null {
  if (!state.shouldShowErrors) {
    return null;
  }
  if (!isConnectionNameProvided(state)) {
    return "Name is required";
  }
  return null;
}

export function validateRequiredFields(
  fields: ConfigField[],
  formValues: Record<string, JsonValue>,
  secretValues: Record<string, string>,
): { field: string; message: string }[] {
  const errors: { field: string; message: string }[] = [];

  for (const field of fields) {
    if (!field.required) continue;

    if (field.type === FieldType.SECRET) {
      const value = secretValues[field.name];
      if (!value || value.trim() === "") {
        errors.push({ field: field.name, message: `${field.name} is required` });
      }
    } else {
      const value = formValues[field.name];
      if (value === undefined || value === null || value === "") {
        errors.push({ field: field.name, message: `${field.name} is required` });
      }
    }
  }

  return errors;
}

/**
 * Get the current value for a field from either formValues or secretValues
 */
export function getFieldValue(
  field: ConfigField,
  formValues: Record<string, JsonValue>,
  secretValues: Record<string, string>,
): JsonValue {
  return field.type === FieldType.SECRET
    ? (secretValues[field.name] ?? null)
    : (formValues[field.name] ?? null);
}

/**
 * Convert a proto Value to JsonValue for form state
 */
export function valueToJsonValue(value: Value | undefined): JsonValue {
  if (!value) return null;
  return toJson(ValueSchema, value);
}

/**
 * Build initial form values from field defaults.
 * SECRET fields are NOT pre-populated from defaults for security reasons.
 */
export function getInitialFormValues(fields: ConfigField[]): Record<string, JsonValue> {
  const values: Record<string, JsonValue> = {};
  for (const field of fields) {
    if (field.default && field.type !== FieldType.SECRET) {
      values[field.name] = valueToJsonValue(field.default);
    }
  }
  return values;
}
