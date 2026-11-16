import { match } from "ts-pattern";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { getConnectorVersionPrefix } from "@/components/connections/utils";

import { CreateConnectionSelectorShelf } from "@/pages/connections/components/create/types";

import { createFilamentFlowSearch } from "@/module/paths";
import type { FilamentLayoutSearch } from "@/module/schemas";
import { createFilamentFlowCloseSearch } from "@/module/utils";

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

const addConnectionToFlowSeed = (
  search: FilamentLayoutSearch,
  connection: Connection,
): FilamentLayoutSearch =>
  match(connection.kind)
    .with(ConnectorKind.SOURCE, () => ({ ...search, sourceId: connection.id }))
    .with(ConnectorKind.SINK, () => ({
      ...search,
      sinkIds: [...(search.sinkIds ?? []), connection.id],
    }))
    .otherwise(() => search);

export const createConnectionCreatedSearch = (
  prev: FilamentLayoutSearch,
  connection: Connection,
): FilamentLayoutSearch =>
  prev.returnFlow
    ? { ...prev, ...createFilamentFlowCloseSearch(addConnectionToFlowSeed(prev, connection)) }
    : { ...prev, ...createFilamentFlowSearch(), connectionId: connection.id };
