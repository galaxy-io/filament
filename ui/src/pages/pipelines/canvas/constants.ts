import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { PlusIcon } from "@phosphor-icons/react";
import { type HandleType, Position } from "@xyflow/react";

import { ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/providers/canvas/types";
import { PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";

export const PIPELINE_CANVAS_FIT_MIN_ZOOM = 0.5;
export const PIPELINE_CANVAS_FIT_MAX_ZOOM = 1;

export const PIPELINE_CANVAS_FIT_INSET_Y = 48;
export const PIPELINE_CANVAS_FIT_INSET_LEFT = 80;

export const PIPELINE_CANVAS_SNAP_GRID: [number, number] = [20, 20];

export const PIPELINE_CANVAS_PAN_ON_DRAG: number[] = [0, 1, 2];

export const PIPELINE_CANVAS_NODE_STACK_START_Y = 100;
export const PIPELINE_CANVAS_NODE_STACK_HEIGHT = 120;
export const PIPELINE_CANVAS_NODE_STACK_GAP = 40;

export const PIPELINE_CANVAS_CONNECTION_SELECTOR_WIDTH = 320;
export const PIPELINE_CANVAS_CONNECTION_SELECTOR_MAX_HEIGHT = 360;

export const CONNECTOR_KIND_TO_NODE_TYPE_MAP: Record<
  ConnectorKind,
  PipelineCanvasNodeType.SOURCE | PipelineCanvasNodeType.SINK
> = {
  [ConnectorKind.UNSPECIFIED]: PipelineCanvasNodeType.SOURCE,
  [ConnectorKind.SOURCE]: PipelineCanvasNodeType.SOURCE,
  [ConnectorKind.SINK]: PipelineCanvasNodeType.SINK,
};

export const CONNECTOR_KIND_TO_HANDLE_TYPE_MAP: Record<ConnectorKind, HandleType> = {
  [ConnectorKind.UNSPECIFIED]: "source",
  [ConnectorKind.SOURCE]: "source",
  [ConnectorKind.SINK]: "target",
};

export const PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP: Record<PipelineCanvasEditMode, PhosphorIcon> = {
  [PipelineCanvasEditMode.ADD_NODE]: PlusIcon,
};

export const PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID = "output";
export const PIPELINE_CANVAS_NODE_SINK_HANDLE_ID = "input";

export const CONNECTOR_KIND_TO_HANDLE_ID_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
  [ConnectorKind.SOURCE]: PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
  [ConnectorKind.SINK]: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
};

export const CONNECTOR_KIND_TO_XYFLOW_POSITION_MAP: Record<ConnectorKind, Position> = {
  [ConnectorKind.UNSPECIFIED]: Position.Right,
  [ConnectorKind.SOURCE]: Position.Right,
  [ConnectorKind.SINK]: Position.Left,
};

export const CONNECTOR_KIND_TO_CHIP_VARIANT_MAP: Record<ConnectorKind, ChipVariant> = {
  [ConnectorKind.UNSPECIFIED]: ChipVariant.LIME,
  [ConnectorKind.SOURCE]: ChipVariant.LIME,
  [ConnectorKind.SINK]: ChipVariant.PINK,
};

export const CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP: Record<ConnectorKind, ConnectorKind> = {
  [ConnectorKind.UNSPECIFIED]: ConnectorKind.SOURCE,
  [ConnectorKind.SOURCE]: ConnectorKind.SOURCE,
  [ConnectorKind.SINK]: ConnectorKind.SINK,
};

export const PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP: Record<
  PipelineCanvasNodeType,
  ConnectorKind
> = {
  [PipelineCanvasNodeType.SOURCE]: ConnectorKind.SOURCE,
  [PipelineCanvasNodeType.SINK]: ConnectorKind.SINK,
  [PipelineCanvasNodeType.PLACEHOLDER]: ConnectorKind.SOURCE,
};

export const PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP: Record<
  PipelineCanvasNodeType.SOURCE | PipelineCanvasNodeType.SINK,
  PipelineCanvasNodeType.SOURCE | PipelineCanvasNodeType.SINK
> = {
  [PipelineCanvasNodeType.SOURCE]: PipelineCanvasNodeType.SINK,
  [PipelineCanvasNodeType.SINK]: PipelineCanvasNodeType.SOURCE,
};

export const PIPELINE_CANVAS_EDGE_TYPE = "pipeline";

export const PIPELINE_CANVAS_EDGE_Z_INDEX = 2000;
export const PIPELINE_CANVAS_OVERLAY_Z_INDEX = 2001;
