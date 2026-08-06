import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { HandIcon, PlusIcon, SelectionIcon } from "@phosphor-icons/react";
import { type FitViewOptions, type HandleType, Position } from "@xyflow/react";

import { ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { ConnectorKind, IngestionType, ReplicationMode } from "@/gen/ingestion/v1/common_pb";

import {
  PipelineCanvasEditMode,
  PipelineCanvasInteractionMode,
} from "@/pages/pipelines/canvas/providers/canvas/types";
import {
  type PipelineCanvasEdgeData,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";

export const PIPELINE_CANVAS_FIT_MIN_ZOOM = 0.5;
export const PIPELINE_CANVAS_FIT_MAX_ZOOM = 1;
export const PIPELINE_CANVAS_FIT_PADDING = 0.5;

export const PIPELINE_CANVAS_FIT_VIEW_OPTIONS: FitViewOptions = {
  padding: PIPELINE_CANVAS_FIT_PADDING,
  maxZoom: PIPELINE_CANVAS_FIT_MAX_ZOOM,
};

export const PIPELINE_CANVAS_SNAP_GRID: [number, number] = [20, 20];

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

export const PIPELINE_CANVAS_INTERACTION_MODE_TO_ICON_MAP: Record<
  PipelineCanvasInteractionMode,
  PhosphorIcon
> = {
  [PipelineCanvasInteractionMode.GRAB]: HandIcon,
  [PipelineCanvasInteractionMode.SELECT]: SelectionIcon,
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

export const PIPELINE_CANVAS_DEFAULT_EDGE_DATA: PipelineCanvasEdgeData = {
  ingestionType: IngestionType.SNAPSHOT_REPLACE,
  selector: "",
  cursors: [],
};

export const INGESTION_TYPE_TO_REPLICATION_MODE_MAP: Record<IngestionType, ReplicationMode> = {
  [IngestionType.UNSPECIFIED]: ReplicationMode.FULL,
  [IngestionType.SNAPSHOT_REPLACE]: ReplicationMode.FULL,
  [IngestionType.SNAPSHOT_UPSERT]: ReplicationMode.FULL,
  [IngestionType.APPEND]: ReplicationMode.FULL,
  [IngestionType.UPSERT]: ReplicationMode.INCREMENTAL,
  [IngestionType.DELETE]: ReplicationMode.INCREMENTAL,
  [IngestionType.CDC]: ReplicationMode.CDC,
};

export const PIPELINE_CANVAS_EDGE_Z_INDEX = 2000;
export const PIPELINE_CANVAS_OVERLAY_Z_INDEX = 2001;
