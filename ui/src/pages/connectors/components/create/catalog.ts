import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

// Catalog aliases identify the default version of a connector family.
export const getConnectorFamily = (connector: ConnectorSpec, catalog: ConnectorSpec[]) =>
  catalog.find(
    (candidate) =>
      candidate.kind === connector.kind &&
      candidate.aliasTarget &&
      (candidate.name === connector.name || connector.name.startsWith(`${candidate.name}@`)),
  ) ?? connector;

export const getConnectorVersions = (family: ConnectorSpec, catalog: ConnectorSpec[]) =>
  family.aliasTarget
    ? catalog.filter(
        (candidate) =>
          candidate.kind === family.kind &&
          !candidate.aliasTarget &&
          candidate.name.startsWith(`${family.name}@`),
      )
    : [];
