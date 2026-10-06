import { match } from "ts-pattern";

import { ConnectorMaturity, type ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { CreateConnectionSelectorShelf } from "@/pages/connectors/components/create/types";

export const getConnectorFamilyName = (name: ConnectorSpec["name"]) => name.split("@")[0];

export const getConnectorVariantName = (name: ConnectorSpec["name"]) => name.split("@")[1];

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

export const isConnectorOnShelf = (
  connector: ConnectorSpec,
  shelf: CreateConnectionSelectorShelf,
): boolean =>
  match(shelf)
    .with(CreateConnectionSelectorShelf.ALL, () => true)
    .with(
      CreateConnectionSelectorShelf.STABLE,
      () => connector.maturity === ConnectorMaturity.STABLE,
    )
    .with(CreateConnectionSelectorShelf.BETA, () => connector.maturity === ConnectorMaturity.BETA)
    .with(CreateConnectionSelectorShelf.ALPHA, () => connector.maturity === ConnectorMaturity.ALPHA)
    .exhaustive();
