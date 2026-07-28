import { create } from "@bufbuild/protobuf";

import { ConnectorKind, IngestionType } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreatePipelineVersionRequest,
  CreatePipelineVersionRequestSchema,
  type PipelineEdge as PipelineEdgeProto,
  type PipelineVersion,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_SNAP_GRID,
  PIPELINE_NODE_SINK_HANDLE_ID,
  PIPELINE_NODE_SOURCE_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import {
  type CanvasEdge,
  type CanvasNode,
  type PipelineCanvasState,
  PipelineNodeType,
  type PipelinePlaceholderNode,
  type PipelineSinkNode,
  type PipelineSourceNode,
} from "@/pages/pipelines/canvas/types";

const NODE_STACK_BASE_X_SOURCE = 100;
const NODE_STACK_BASE_X_SINK = 500;
const NODE_STACK_START_Y = 100;
const NODE_STACK_HEIGHT = 120;
const NODE_STACK_GAP = 40;

const CONNECTOR_KIND_TO_NODE_TYPE_MAP: Record<
  ConnectorKind,
  PipelineNodeType.SOURCE | PipelineNodeType.SINK
> = {
  [ConnectorKind.UNSPECIFIED]: PipelineNodeType.SOURCE,
  [ConnectorKind.SOURCE]: PipelineNodeType.SOURCE,
  [ConnectorKind.SINK]: PipelineNodeType.SINK,
};

const getNormalizedConnectorKind = (kind: ConnectorKind) =>
  kind === ConnectorKind.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE;

export const getNextNodePosition = (kind: ConnectorKind, nodes: CanvasNode[]) => {
  const nodeType = CONNECTOR_KIND_TO_NODE_TYPE_MAP[kind];
  const baseX =
    nodeType === PipelineNodeType.SINK ? NODE_STACK_BASE_X_SINK : NODE_STACK_BASE_X_SOURCE;

  const sameTypeNodes = nodes.filter((node) => node.type === nodeType);

  if (sameTypeNodes.length === 0) {
    return { x: baseX, y: NODE_STACK_START_Y };
  }

  const maxY = sameTypeNodes.reduce((max, node) => Math.max(max, node.position.y), 0);

  const nextY = maxY + NODE_STACK_HEIGHT + NODE_STACK_GAP;
  const [, snapY] = PIPELINE_CANVAS_SNAP_GRID;

  return { x: baseX, y: Math.round(nextY / snapY) * snapY };
};

const buildPlaceholderNode = (kind: ConnectorKind): PipelinePlaceholderNode => ({
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

export const getPlaceholderNodes = (nodes: CanvasNode[], isReadOnly: boolean): CanvasNode[] => {
  if (isReadOnly) return [];

  const placeholders: CanvasNode[] = [];
  if (!nodes.some((node) => node.type === PipelineNodeType.SOURCE)) {
    placeholders.push(SOURCE_PLACEHOLDER_NODE);
  }
  if (!nodes.some((node) => node.type === PipelineNodeType.SINK)) {
    placeholders.push(SINK_PLACEHOLDER_NODE);
  }
  return placeholders;
};

export const mapNodesToStackedPositions = (nodes: CanvasNode[]): CanvasNode[] => {
  const repositioned: CanvasNode[] = [];
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
): CanvasNode => {
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

const isConnectionNode = (node: CanvasNode): node is PipelineSourceNode | PipelineSinkNode =>
  node.type === PipelineNodeType.SOURCE || node.type === PipelineNodeType.SINK;

const getConnectorKind = (node: PipelineSourceNode | PipelineSinkNode) =>
  node.type === PipelineNodeType.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE;

const getCanvasEdgeResource = (edge: CanvasEdge) =>
  edge.sourceHandle && edge.sourceHandle !== PIPELINE_NODE_SOURCE_HANDLE_ID
    ? edge.sourceHandle
    : "";

const getProtoEdgeKey = (edge: PipelineEdgeProto) =>
  `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

const getCanvasEdgeKey = (edge: CanvasEdge) =>
  `${edge.source}|${getCanvasEdgeResource(edge)}|${edge.target}`;

export const mapPipelineVersionToCanvasState = (
  version: PipelineVersion | undefined,
  connections: Connection[],
): Pick<PipelineCanvasState, "nodes" | "edges"> => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));

  const nodes: CanvasNode[] = [];
  for (const node of version?.nodes ?? []) {
    const connection = connectionsById.get(node.connectionId);
    const kind = getNormalizedConnectorKind(node.kind);
    const type = CONNECTOR_KIND_TO_NODE_TYPE_MAP[kind];

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

  const edges: CanvasEdge[] = (version?.edges ?? []).map((edge) => ({
    id: getProtoEdgeKey(edge),
    type: PIPELINE_CANVAS_EDGE_TYPE,
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
    .map((node) => `${node.id}|${getNormalizedConnectorKind(node.kind)}|${node.connectionId}`)
    .sort();

  const canvasEdges = state.edges.map(getCanvasEdgeKey).sort();
  const pipelineEdges = (version?.edges ?? []).map(getProtoEdgeKey).sort();

  return (
    canvasNodes.join(",") !== pipelineNodes.join(",") ||
    canvasEdges.join(",") !== pipelineEdges.join(",")
  );
};
