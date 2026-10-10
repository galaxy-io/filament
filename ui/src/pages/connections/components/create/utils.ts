import { match } from "ts-pattern";

import { ConnectorMaturity, type ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { getConnectorVersionPrefix } from "@/components/connections/utils";

import { CreateConnectionSelectorShelf } from "@/pages/connections/components/create/types";

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
