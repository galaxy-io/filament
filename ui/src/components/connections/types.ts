export enum ConnectorTileSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

export enum ConnectorTileBadge {
  WEBHOOKS = "WEBHOOKS",
}

export interface ConnectorTileBadgeGeometry {
  size: number;
  iconSize: number;
  offset: number;
}
