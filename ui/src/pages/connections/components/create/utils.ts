import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { getConnectorVersionPrefix } from "@/components/connections/utils";

import type { CreateConnectionSelectorShelf } from "@/pages/connections/components/create/types";
import { CREATE_CONNECTION_SELECTOR_SHELF_TO_MATURITY_MAP } from "@/pages/connections/constants";

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
): boolean => {
  const maturity = CREATE_CONNECTION_SELECTOR_SHELF_TO_MATURITY_MAP[shelf];
  return maturity === undefined || connector.maturity === maturity;
};
