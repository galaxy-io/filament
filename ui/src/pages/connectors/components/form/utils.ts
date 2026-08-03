import type { JsonObject } from "@bufbuild/protobuf";

import { type ConfigField, FieldType } from "@/gen/ingestion/v1/common_pb";

import { isJsonObject } from "@/components/fields/utils";

export function omitBlankSecretFields(fields: ConfigField[], config: JsonObject): JsonObject {
  const result: JsonObject = { ...config };
  for (const field of fields) {
    const value = result[field.name];
    if (field.type === FieldType.SECRET || field.secret) {
      if (value === "" || value === null || value === undefined) {
        delete result[field.name];
      }
      continue;
    }
    if (field.type === FieldType.OBJECT && field.fields.length > 0 && isJsonObject(value)) {
      result[field.name] = omitBlankSecretFields(field.fields, value);
    }
  }
  return result;
}
