import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

// Layout constants
export const CONNECTOR_SEARCH_WIDTH = 280;
export const CONNECTOR_GRID_MIN_COLUMN_WIDTH = 320;
export const CONNECTOR_DRAWER_WIDTH = 600;

// Maps
export const CONNECTOR_KIND_TO_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "All connectors",
  [ConnectorKind.SOURCE]: "Sources",
  [ConnectorKind.SINK]: "Sinks",
};
