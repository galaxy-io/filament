import { type JsonValue, toJson } from "@bufbuild/protobuf";
import { ValueSchema } from "@bufbuild/protobuf/wkt";

import { type ConfigField, FieldScope } from "@/gen/ingestion/v1/common_pb";

const ACRONYMS_TO_CAPITALIZE: string[] = [
  "api",
  "url",
  "id",
  "s3",
  "aws",
  "sql",
  "json",
  "http",
  "ssh",
  "ssl",
  "dsn",
  "uri",
  "kb",
  "mb",
  "gb",
  "tb",
  "kib",
  "mib",
  "gib",
  "tib",
];

export function formatFieldName(fieldName: string): string {
  return fieldName
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .split(" ")
    .map((word) => {
      if (ACRONYMS_TO_CAPITALIZE.includes(word.toLowerCase())) {
        return word.toUpperCase();
      }
      return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
    })
    .join(" ");
}

export function isFieldVisible(field: ConfigField, siblings: Record<string, JsonValue>): boolean {
  if (!field.visibleWhen) return true;

  const value = siblings[field.visibleWhen.field];
  return typeof value === "string" && field.visibleWhen.values.includes(value);
}

export function isJsonObject(value: JsonValue): value is Record<string, JsonValue> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function getFieldDefaults(fields: ConfigField[]): Record<string, JsonValue> {
  const defaults: Record<string, JsonValue> = {};
  for (const field of fields) {
    if (field.default) defaults[field.name] = toJson(ValueSchema, field.default);
  }
  return defaults;
}

export function getConnectionScopedFields(fields: ConfigField[]): ConfigField[] {
  return fields.filter((field) => field.scope === FieldScope.CONNECTION);
}

export function getPipelineScopedFields(fields: ConfigField[]): ConfigField[] {
  return fields.filter((field) => field.scope === FieldScope.PIPELINE);
}
