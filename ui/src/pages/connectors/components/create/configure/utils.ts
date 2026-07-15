import { create, toJson, type JsonValue } from "@bufbuild/protobuf";
import { ValueSchema, type Value } from "@bufbuild/protobuf/wkt";

import {
  FieldScope,
  FieldType,
  type ConfigField,
} from "@/gen/ingestion/v1/common_pb";
import {
  CreateConnectionRequestSchema,
  type CreateConnectionRequest,
} from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export function getConnectorConfigSchemaConnectionFields(
  connector: ConnectorSpec,
): ConfigField[] {
  return (
    connector.configSchema?.fields?.filter(
      (f) => f.scope === FieldScope.CONNECTION,
    ) ?? []
  );
}

export function generateSecretRef(
  connectionName: string,
  fieldName: string,
): string {
  const timestamp = Date.now();
  const sanitizedName = connectionName
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-");
  return `${sanitizedName}-${fieldName}-${timestamp}`;
}

/**
 * Convert a proto Value to JsonValue for form state
 */
export function valueToJsonValue(value: Value | undefined): JsonValue {
  if (!value) return null;
  return toJson(ValueSchema, value);
}

/**
 * Initialize a CreateConnectionRequest from connector spec.
 */
export function createInitialCreateConnectionRequest(
  connector: ConnectorSpec,
): CreateConnectionRequest {
  const fields = getConnectorConfigSchemaConnectionFields(connector);
  const initialConfig: Record<string, JsonValue> = {};

  for (const field of fields) {
    if (field.default && field.type !== FieldType.SECRET) {
      initialConfig[field.name] = valueToJsonValue(field.default);
    }
  }

  return create(CreateConnectionRequestSchema, {
    kind: connector.kind,
    connector: connector.name,
    name: "",
    config: initialConfig,
    secretRefs: {},
  });
}
