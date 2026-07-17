import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { HandIcon, PencilSimpleIcon, PlusIcon, PulseIcon } from "@phosphor-icons/react";
import type { FitViewOptions, HandleType } from "@xyflow/react";

import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export const CANVAS_FIT_VIEW_OPTIONS: FitViewOptions = {
  padding: 0.4,
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
  [PipelineCanvasEditMode.ADD_EDGE]: PencilSimpleIcon,
  [PipelineCanvasEditMode.GRAB]: HandIcon,
  [PipelineCanvasEditMode.ACTIVITY]: PulseIcon,
};

export const PIPELINE_NODE_WIDTH = 300;
export const PIPELINE_NODE_PADDING = 8;
export const PIPELINE_NODE_BORDER_RADIUS = 6;
export const PIPELINE_NODE_GAP = 8;

export const PIPELINE_NODE_HANDLE_SLOT_SIZE = 24;

export const PIPELINE_NODE_SOURCE_HANDLE_ID = "output";
export const PIPELINE_NODE_SINK_HANDLE_ID = "input";

export const CANVAS_TERMINAL_WIDTH = 420;
export const CANVAS_TERMINAL_MAX_HEIGHT = 280;
