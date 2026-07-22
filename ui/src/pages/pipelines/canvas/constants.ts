import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { HandIcon, PlusIcon, SelectionIcon } from "@phosphor-icons/react";
import type { FitViewOptions, HandleType } from "@xyflow/react";

import {
  PipelineCanvasEditMode,
  PipelineCanvasInteractionMode,
} from "@/pages/pipelines/canvas/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export const CANVAS_FIT_VIEW_OPTIONS: FitViewOptions = {
  padding: 0.5,
  maxZoom: 1,
};

export const CANVAS_SNAP_GRID: [number, number] = [20, 20];

export const CONNECTOR_KIND_TO_HANDLE_TYPE_MAP: Record<ConnectorKind, HandleType> = {
  [ConnectorKind.UNSPECIFIED]: "source",
  [ConnectorKind.SOURCE]: "source",
  [ConnectorKind.SINK]: "target",
};

export const PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP: Record<PipelineCanvasEditMode, PhosphorIcon> = {
  [PipelineCanvasEditMode.ADD_NODE]: PlusIcon,
};

export const PIPELINE_CANVAS_INTERACTION_MODE_TO_ICON_MAP: Record<
  PipelineCanvasInteractionMode,
  PhosphorIcon
> = {
  [PipelineCanvasInteractionMode.GRAB]: HandIcon,
  [PipelineCanvasInteractionMode.SELECT]: SelectionIcon,
};

export const PIPELINE_NODE_WIDTH = 360;
export const PIPELINE_NODE_PADDING = 8;
export const PIPELINE_NODE_BORDER_RADIUS = 6;
export const PIPELINE_NODE_GAP = 8;

export const PIPELINE_NODE_HANDLE_SLOT_SIZE = 24;
export const PIPELINE_NODE_TABLE_LIST_MAX_HEIGHT = 300;

export const PIPELINE_NODE_SOURCE_HANDLE_ID = "output";
export const PIPELINE_NODE_SINK_HANDLE_ID = "input";

export const PIPELINE_EDGE_TYPE = "pipeline";

export const CANVAS_TERMINAL_WIDTH = 640;
export const CANVAS_TERMINAL_HEIGHT = 640;
export const CANVAS_TERMINAL_RIGHT_OFFSET = 24;
export const CANVAS_TERMINAL_NOTCH_WIDTH = 360;
