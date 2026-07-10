import type { FitViewOptions, HandleType } from "@xyflow/react";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";

import {
  ConnectorType,
  PipelineNodeType,
  type PipelineNode,
  type PipelineEdge,
} from "@/pages/pipelines/canvas/types";

// Canvas configuration
export const CANVAS_FIT_VIEW_OPTIONS: FitViewOptions = {
  padding: 0.4,
  maxZoom: 1,
};

// Snap to grid configuration
export const CANVAS_SNAP_GRID: [number, number] = [20, 20];

// Maps ProviderKind to React Flow handle type
export const PROVIDER_KIND_TO_HANDLE_TYPE_MAP: Record<
  ProviderKind,
  HandleType
> = {
  [ProviderKind.UNSPECIFIED]: "source",
  [ProviderKind.SOURCE]: "source",
  [ProviderKind.SINK]: "target",
};

// Shared node dimensions
export const PIPELINE_NODE_WIDTH = 300;
export const PIPELINE_NODE_SINK_WIDTH = 240;
export const PIPELINE_NODE_PADDING = 8;
export const PIPELINE_NODE_BORDER_RADIUS = 6;
export const PIPELINE_NODE_GAP = 10;

// Handle slot dimensions
export const PIPELINE_NODE_HANDLE_SLOT_SIZE = 24;

// Demo data for initial canvas
export const DEMO_INITIAL_NODES: PipelineNode[] = [
  {
    id: "source-1",
    type: PipelineNodeType.SOURCE,
    position: { x: 100, y: 200 },
    data: {
      label: "postgres",
      connectorType: ConnectorType.POSTGRES,
      tables: [
        { name: "lineitem", rowCount: "6.0M rows", isConnected: false },
        { name: "nation", rowCount: "25 rows", isConnected: true },
        { name: "orders", rowCount: "1.5M rows", isConnected: false },
        { name: "part", rowCount: "200k rows", isConnected: true },
        { name: "partsupp", rowCount: "800k rows", isConnected: false },
        { name: "region", rowCount: "5 rows", isConnected: true },
        { name: "supplier", rowCount: "10k rows", isConnected: false },
      ],
    },
  },
  {
    id: "sink-1",
    type: PipelineNodeType.SINK,
    position: { x: 700, y: 100 },
    data: {
      label: "postgres",
      connectorType: ConnectorType.POSTGRES,
    },
  },
  {
    id: "sink-2",
    type: PipelineNodeType.SINK,
    position: { x: 700, y: 300 },
    data: {
      label: "s3",
      connectorType: ConnectorType.S3,
    },
  },
  {
    id: "sink-3",
    type: PipelineNodeType.SINK,
    position: { x: 700, y: 500 },
    data: {
      label: "stdout",
      connectorType: ConnectorType.STDOUT,
    },
  },
];

export const DEMO_INITIAL_EDGES: PipelineEdge[] = [
  {
    id: "edge-1",
    source: "source-1",
    sourceHandle: "nation",
    target: "sink-1",
    targetHandle: "input",
  },
  {
    id: "edge-2",
    source: "source-1",
    sourceHandle: "part",
    target: "sink-2",
    targetHandle: "input",
  },
  {
    id: "edge-3",
    source: "source-1",
    sourceHandle: "region",
    target: "sink-3",
    targetHandle: "input",
  },
];
