import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import {
  CONNECTOR_TILE_BADGE_GAP,
  CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP,
} from "@/components/connections/constants";
import type { ConnectorTileSize } from "@/components/connections/types";

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

export const formatConnectorName = ({
  displayName,
  name,
}: Pick<ConnectorSpec, "displayName" | "name">) => displayName || name;

export const createConnectorTileBadgeMask = (size: ConnectorTileSize) => {
  const { size: badgeSize, offset } = CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP[size];
  const radius = badgeSize / 2;
  const center = radius - offset;
  const notch = radius + CONNECTOR_TILE_BADGE_GAP;
  return `radial-gradient(circle at calc(100% - ${center}px) calc(100% - ${center}px), transparent ${notch}px, black ${notch + 0.5}px)`;
};
