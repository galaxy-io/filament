import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

export const getConnectorFamilyName = (name: ConnectorSpec["name"]) => name.split("@")[0];

export const getConnectorVariantName = (name: ConnectorSpec["name"]) => name.split("@")[1];

export const getConnectorFamily = (connector: ConnectorSpec, catalog: ConnectorSpec[]) =>
  catalog.find(
    (candidate) =>
      candidate.kind === connector.kind &&
      candidate.aliasTarget &&
      (candidate.name === connector.name ||
        connector.name.startsWith(getConnectorVersionPrefix(candidate))),
  ) ?? connector;

export const getConnectorVersionPrefix = (family: ConnectorSpec) =>
  `${getConnectorFamilyName(family.aliasTarget)}@`;

export const mapConnectorsToShuffled = (connectors: ConnectorSpec[]) => {
  const shuffled = [...connectors];
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const swap = Math.floor(Math.random() * (index + 1));
    [shuffled[index], shuffled[swap]] = [shuffled[swap], shuffled[index]];
  }
  return shuffled;
};
