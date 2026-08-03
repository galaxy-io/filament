import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export const PIPELINE_CANVAS_NODE_WIDTH = 360;
export const PIPELINE_CANVAS_NODE_PADDING = 8;
export const PIPELINE_CANVAS_NODE_BORDER_RADIUS = 6;
export const PIPELINE_CANVAS_NODE_GAP = 8;

export const PIPELINE_CANVAS_NODE_PLACEHOLDER_SELECTOR_HEIGHT = 300;

export const PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE = 24;
export const PIPELINE_CANVAS_NODE_TABLE_LIST_MAX_HEIGHT = 300;

export const CONNECTOR_KIND_TO_PLACEHOLDER_TITLE_MAP: Record<
  ConnectorKind,
  string
> = {
  [ConnectorKind.UNSPECIFIED]: "Select a source",
  [ConnectorKind.SOURCE]: "Select a source",
  [ConnectorKind.SINK]: "Select a sink",
};

export const CONNECTOR_KIND_TO_PLACEHOLDER_DESCRIPTION_MAP: Record<
  ConnectorKind,
  string
> = {
  [ConnectorKind.UNSPECIFIED]: "A source is where data comes from.",
  [ConnectorKind.SOURCE]: "A source is where data comes from.",
  [ConnectorKind.SINK]: "A sink is where data goes to.",
};
