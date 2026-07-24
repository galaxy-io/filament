import type { JsonValue } from "@bufbuild/protobuf";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

export function isFieldVisible(field: ConfigField, siblings: Record<string, JsonValue>): boolean {
  if (!field.visibleWhen) return true;

  const value = siblings[field.visibleWhen.field];
  return typeof value === "string" && field.visibleWhen.values.includes(value);
}
