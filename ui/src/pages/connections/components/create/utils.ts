import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

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
): boolean => {
  return shelf === CreateConnectionSelectorShelf.ALL || connector.isPreview;
};
