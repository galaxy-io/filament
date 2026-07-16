import { type ConfigField, FieldScope } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export function getConnectorConfigSchemaConnectionFields(connector: ConnectorSpec): ConfigField[] {
  return connector.configSchema?.fields?.filter((f) => f.scope === FieldScope.CONNECTION) ?? [];
}
