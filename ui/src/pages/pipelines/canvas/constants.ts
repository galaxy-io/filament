import type { FitViewOptions, HandleType } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

// Canvas configuration
export const CANVAS_FIT_VIEW_OPTIONS: FitViewOptions = {
  padding: 0.4,
  maxZoom: 1,
};

// Snap to grid configuration
export const CANVAS_SNAP_GRID: [number, number] = [20, 20];

// Maps ConnectorKind to React Flow handle type
export const CONNECTOR_KIND_TO_HANDLE_TYPE_MAP: Record<
  ConnectorKind,
  HandleType
> = {
  [ConnectorKind.UNSPECIFIED]: "source",
  [ConnectorKind.SOURCE]: "source",
  [ConnectorKind.SINK]: "target",
};

// Shared node dimensions
export const PIPELINE_NODE_WIDTH = 300;
export const PIPELINE_NODE_SINK_WIDTH = 240;
export const PIPELINE_NODE_PADDING = 8;
export const PIPELINE_NODE_BORDER_RADIUS = 6;
export const PIPELINE_NODE_GAP = 10;

// Handle slot dimensions
export const PIPELINE_NODE_HANDLE_SLOT_SIZE = 24;
