import { create } from "@bufbuild/protobuf";

import {
  CANVAS_SNAP_GRID,
  PIPELINE_EDGE_TYPE,
  PIPELINE_NODE_SINK_HANDLE_ID,
  PIPELINE_NODE_SOURCE_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import {
  type PipelineCanvasState,
  type PipelineEdge,
  type PipelineNode,
  type PipelineNodePlaceholder,
  type PipelineNodeSink,
  type PipelineNodeSource,
  PipelineNodeType,
} from "@/pages/pipelines/canvas/types";

import { ConnectorKind, IngestionType } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreatePipelineVersionRequest,
  CreatePipelineVersionRequestSchema,
  type PipelineEdge as PipelineEdgeProto,
  type PipelineVersion,
} from "@/gen/ingestion/v1/pipelines_pb";

const NODE_STACK_BASE_X_SOURCE = 100;
const NODE_STACK_BASE_X_SINK = 500;
const NODE_STACK_START_Y = 100;
const NODE_STACK_HEIGHT = 120;
const NODE_STACK_GAP = 40;

export const getNextNodePosition = (kind: ConnectorKind, nodes: PipelineNode[]) => {
  const isSource = kind === ConnectorKind.SOURCE;
  const baseX = isSource ? NODE_STACK_BASE_X_SOURCE : NODE_STACK_BASE_X_SINK;
  const nodeType = isSource ? PipelineNodeType.SOURCE : PipelineNodeType.SINK;

  const sameTypeNodes = nodes.filter((node) => node.type === nodeType);

  if (sameTypeNodes.length === 0) {
    return { x: baseX, y: NODE_STACK_START_Y };
  }

  const maxY = sameTypeNodes.reduce((max, node) => Math.max(max, node.position.y), 0);

  const nextY = maxY + NODE_STACK_HEIGHT + NODE_STACK_GAP;
  const [, snapY] = CANVAS_SNAP_GRID;

  return { x: baseX, y: Math.round(nextY / snapY) * snapY };
};

// Empty-state ghosts: virtual nodes injected at render time, never stored in the
// reducer, so they can't be saved, diffed, or re-stacked. Module-level constants
// keep references stable across renders so React Flow doesn't re-measure them.
const buildPlaceholderNode = (kind: ConnectorKind): PipelineNodePlaceholder => ({
  id: kind === ConnectorKind.SOURCE ? "placeholder-source" : "placeholder-sink",
  type: PipelineNodeType.PLACEHOLDER,
  position: {
    x: kind === ConnectorKind.SOURCE ? NODE_STACK_BASE_X_SOURCE : NODE_STACK_BASE_X_SINK,
    y: NODE_STACK_START_Y,
  },
  data: { kind },
  draggable: false,
  selectable: false,
  deletable: false,
  connectable: false,
});

const SOURCE_PLACEHOLDER_NODE = buildPlaceholderNode(ConnectorKind.SOURCE);
const SINK_PLACEHOLDER_NODE = buildPlaceholderNode(ConnectorKind.SINK);

export const getPlaceholderNodes = (nodes: PipelineNode[], isReadOnly: boolean): PipelineNode[] => {
  if (isReadOnly) return [];

  const placeholders: PipelineNode[] = [];
  if (!nodes.some((node) => node.type === PipelineNodeType.SOURCE)) {
    placeholders.push(SOURCE_PLACEHOLDER_NODE);
  }
  if (!nodes.some((node) => node.type === PipelineNodeType.SINK)) {
    placeholders.push(SINK_PLACEHOLDER_NODE);
  }
  return placeholders;
};

// Re-stack every node into the same deterministic layout a fresh load produces
export const resetNodePositions = (nodes: PipelineNode[]): PipelineNode[] => {
  const repositioned: PipelineNode[] = [];
  for (const node of nodes) {
    const kind = node.type === PipelineNodeType.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE;
    repositioned.push({
      ...node,
      position: getNextNodePosition(kind, repositioned),
      selected: false,
    });
  }
  return repositioned;
};

export const createNodeFromConnection = (
  connection: Connection,
  position: { x: number; y: number },
): PipelineNode => {
  const data = {
    label: connection.name,
    connector: connection.connector,
    connectionId: connection.id,
  };

  if (connection.kind === ConnectorKind.SOURCE) {
    return { id: crypto.randomUUID(), type: PipelineNodeType.SOURCE, position, data };
  }

  return { id: crypto.randomUUID(), type: PipelineNodeType.SINK, position, data };
};

// Only SOURCE/SINK nodes round-trip to the backend (the union also allows BuiltInNode)
const isConnectionNode = (node: PipelineNode): node is PipelineNodeSource | PipelineNodeSink =>
  node.type === PipelineNodeType.SOURCE || node.type === PipelineNodeType.SINK;

const getConnectorKind = (node: PipelineNodeSource | PipelineNodeSink) =>
  node.type === PipelineNodeType.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE;

// The "output" handle routes all enabled resources, which the proto encodes as ""
const getCanvasEdgeResource = (edge: PipelineEdge) =>
  edge.sourceHandle && edge.sourceHandle !== PIPELINE_NODE_SOURCE_HANDLE_ID
    ? edge.sourceHandle
    : "";

// Stable edge identity, matching the backend's "from|resource|to" convention
const getProtoEdgeKey = (edge: PipelineEdgeProto) =>
  `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

const getCanvasEdgeKey = (edge: PipelineEdge) =>
  `${edge.source}|${getCanvasEdgeResource(edge)}|${edge.target}`;

// A pipeline with no saved versions yet maps to an empty canvas
export const mapPipelineVersionToCanvasState = (
  version: PipelineVersion | undefined,
  connections: Connection[],
): Pick<PipelineCanvasState, "nodes" | "edges"> => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));

  // Positions are not persisted (no proto field) - stack deterministically as nodes accumulate
  const nodes: PipelineNode[] = [];
  for (const node of version?.nodes ?? []) {
    const connection = connectionsById.get(node.connectionId);
    const kind = node.kind === ConnectorKind.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE;
    const type = kind === ConnectorKind.SINK ? PipelineNodeType.SINK : PipelineNodeType.SOURCE;

    nodes.push({
      id: node.id,
      type,
      position: getNextNodePosition(kind, nodes),
      data: {
        label: connection?.name ?? node.connectionId,
        connector: connection?.connector ?? "",
        connectionId: node.connectionId,
      },
    });
  }

  const edges: PipelineEdge[] = (version?.edges ?? []).map((edge) => ({
    id: getProtoEdgeKey(edge),
    type: PIPELINE_EDGE_TYPE,
    source: edge.fromNode,
    target: edge.toNode,
    sourceHandle: edge.resource || PIPELINE_NODE_SOURCE_HANDLE_ID,
    targetHandle: PIPELINE_NODE_SINK_HANDLE_ID,
  }));

  return { nodes, edges };
};

export const mapCanvasStateToVersionRequest = (
  state: PipelineCanvasState,
  pipelineId: string,
  baseVersion: PipelineVersion | undefined,
): CreatePipelineVersionRequest => {
  const baseNodesById = new Map((baseVersion?.nodes ?? []).map((node) => [node.id, node]));
  const baseEdgesByKey = new Map(
    (baseVersion?.edges ?? []).map((edge) => [getProtoEdgeKey(edge), edge]),
  );

  // Preserve fields the canvas doesn't edit (config overlays, ingestion settings)
  const nodes = state.nodes.filter(isConnectionNode).map((node) => {
    const baseNode = baseNodesById.get(node.id);
    return {
      id: node.id,
      kind: getConnectorKind(node),
      connectionId: node.data.connectionId,
      config: baseNode?.config,
      secretRefs: baseNode?.secretRefs ?? {},
    };
  });

  const edges = state.edges.map((edge) => {
    const baseEdge = baseEdgesByKey.get(getCanvasEdgeKey(edge));
    return {
      fromNode: edge.source,
      resource: getCanvasEdgeResource(edge),
      toNode: edge.target,
      ingestionType: baseEdge?.ingestionType ?? IngestionType.UNSPECIFIED,
      selector: baseEdge?.selector ?? "",
    };
  });

  return create(CreatePipelineVersionRequestSchema, {
    pipelineId,
    nodes,
    edges,
  });
};

// A pipeline can only run once it has a source, a sink, and at least one route between them
export const isPipelineRunnable = (version: PipelineVersion | undefined): boolean =>
  (version?.nodes ?? []).some((node) => node.kind === ConnectorKind.SOURCE) &&
  (version?.nodes ?? []).some((node) => node.kind === ConnectorKind.SINK) &&
  (version?.edges ?? []).length > 0;

export const hasPipelineGraphChanges = (
  state: PipelineCanvasState,
  version: PipelineVersion | undefined,
): boolean => {
  const canvasNodes = state.nodes
    .filter(isConnectionNode)
    .map((node) => `${node.id}|${getConnectorKind(node)}|${node.data.connectionId}`)
    .sort();
  const pipelineNodes = (version?.nodes ?? [])
    .map(
      (node) =>
        `${node.id}|${node.kind === ConnectorKind.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE}|${node.connectionId}`,
    )
    .sort();

  const canvasEdges = state.edges.map(getCanvasEdgeKey).sort();
  const pipelineEdges = (version?.edges ?? []).map(getProtoEdgeKey).sort();

  return (
    canvasNodes.join(",") !== pipelineNodes.join(",") ||
    canvasEdges.join(",") !== pipelineEdges.join(",")
  );
};
