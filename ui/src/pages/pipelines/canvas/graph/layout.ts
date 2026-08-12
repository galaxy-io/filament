import { getNodesBounds, type Rect } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import {
  CONNECTOR_KIND_TO_NODE_TYPE_MAP,
  PIPELINE_CANVAS_NODE_STACK_GAP,
  PIPELINE_CANVAS_NODE_STACK_HEIGHT,
  PIPELINE_CANVAS_NODE_STACK_START_Y,
  PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP,
  PIPELINE_CANVAS_SNAP_GRID,
} from "@/pages/pipelines/canvas/constants";
import { PIPELINE_CANVAS_NODE_WIDTH } from "@/pages/pipelines/canvas/nodes/constants";
import {
  type CanvasNode,
  PipelineCanvasNodeType,
  type PipelineCanvasPlaceholderNode,
} from "@/pages/pipelines/canvas/types";

const CONNECTOR_KIND_TO_NODE_STACK_BASE_X_MAP: Record<ConnectorKind, number> = {
  [ConnectorKind.UNSPECIFIED]: 100,
  [ConnectorKind.SOURCE]: 100,
  [ConnectorKind.SINK]: 500,
};

const CONNECTOR_KIND_TO_PLACEHOLDER_ID_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "placeholder-source",
  [ConnectorKind.SOURCE]: "placeholder-source",
  [ConnectorKind.SINK]: "placeholder-sink",
};

export const getNextNodePosition = (kind: ConnectorKind, nodes: CanvasNode[]) => {
  const nodeType = CONNECTOR_KIND_TO_NODE_TYPE_MAP[kind];
  const baseX = CONNECTOR_KIND_TO_NODE_STACK_BASE_X_MAP[kind];

  const sameTypeNodes = nodes.filter((node) => node.type === nodeType);

  if (sameTypeNodes.length === 0) {
    return { x: baseX, y: PIPELINE_CANVAS_NODE_STACK_START_Y };
  }

  const maxY = sameTypeNodes.reduce((max, node) => Math.max(max, node.position.y), 0);

  const nextY = maxY + PIPELINE_CANVAS_NODE_STACK_HEIGHT + PIPELINE_CANVAS_NODE_STACK_GAP;
  const [, snapY] = PIPELINE_CANVAS_SNAP_GRID;

  return { x: baseX, y: Math.round(nextY / snapY) * snapY };
};

const buildPlaceholderNode = (kind: ConnectorKind): PipelineCanvasPlaceholderNode => ({
  id: CONNECTOR_KIND_TO_PLACEHOLDER_ID_MAP[kind],
  type: PipelineCanvasNodeType.PLACEHOLDER,
  position: {
    x: CONNECTOR_KIND_TO_NODE_STACK_BASE_X_MAP[kind],
    y: PIPELINE_CANVAS_NODE_STACK_START_Y,
  },
  data: { kind },
  draggable: false,
  selectable: false,
  deletable: false,
  connectable: false,
  style: { pointerEvents: "all" },
});

const SOURCE_PLACEHOLDER_NODE = buildPlaceholderNode(ConnectorKind.SOURCE);
const SINK_PLACEHOLDER_NODE = buildPlaceholderNode(ConnectorKind.SINK);

export const getPlaceholderNodes = (nodes: CanvasNode[], isReadOnly: boolean): CanvasNode[] => {
  if (isReadOnly) return [];

  const placeholders: CanvasNode[] = [];
  if (!nodes.some((node) => node.type === PipelineCanvasNodeType.SOURCE)) {
    placeholders.push(SOURCE_PLACEHOLDER_NODE);
  }
  if (!nodes.some((node) => node.type === PipelineCanvasNodeType.SINK)) {
    placeholders.push(SINK_PLACEHOLDER_NODE);
  }
  return placeholders;
};

export const getGraphBounds = (nodes: CanvasNode[]): Rect =>
  getNodesBounds(
    nodes.map((node) => ({
      ...node,
      measured: {
        width: node.measured?.width ?? PIPELINE_CANVAS_NODE_WIDTH,
        height: node.measured?.height ?? PIPELINE_CANVAS_NODE_STACK_HEIGHT,
      },
    })),
  );

export const mapNodesToStackedPositions = (nodes: CanvasNode[]): CanvasNode[] => {
  const repositioned: CanvasNode[] = [];
  for (const node of nodes) {
    const kind = PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type];
    repositioned.push({
      ...node,
      position: getNextNodePosition(kind, repositioned),
      selected: false,
    });
  }
  return repositioned;
};
