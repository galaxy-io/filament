import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { FilamentNavItem } from "@/module/types";

export const FILAMENT_COMMAND_GROUP_GO_TO = "Go to";
export const FILAMENT_COMMAND_GROUP_CREATE = "Create";
export const FILAMENT_COMMAND_GROUP_PIPELINES = "Pipelines";
export const FILAMENT_COMMAND_GROUP_CONNECTIONS = "Connections";

export const FILAMENT_CONNECTOR_KIND_TO_NAV_ITEM_MAP: Record<ConnectorKind, FilamentNavItem> = {
  [ConnectorKind.UNSPECIFIED]: FilamentNavItem.SOURCES,
  [ConnectorKind.SOURCE]: FilamentNavItem.SOURCES,
  [ConnectorKind.SINK]: FilamentNavItem.SINKS,
};

export const FILAMENT_CONNECTOR_KIND_TO_CREATE_COMMAND_ID_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "new-connection",
  [ConnectorKind.SOURCE]: "new-source",
  [ConnectorKind.SINK]: "new-sink",
};
