import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

export const getConnectorFamilyName = (name: ConnectorSpec["name"]) => name.split("@")[0];

const getConnectorVersionPrefix = (family: ConnectorSpec) =>
  `${getConnectorFamilyName(family.aliasTarget)}@`;

export const getConnectorFamily = (connector: ConnectorSpec, catalog: ConnectorSpec[]) =>
  catalog.find(
    (candidate) =>
      candidate.kind === connector.kind &&
      candidate.aliasTarget &&
      (candidate.name === connector.name ||
        connector.name.startsWith(getConnectorVersionPrefix(candidate))),
  ) ?? connector;

export const getConnectorVersions = (family: ConnectorSpec, catalog: ConnectorSpec[]) =>
  family.aliasTarget
    ? catalog.filter(
        (candidate) =>
          candidate.kind === family.kind &&
          !candidate.aliasTarget &&
          candidate.name.startsWith(getConnectorVersionPrefix(family)),
      )
    : [];
