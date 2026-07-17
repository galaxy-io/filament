import { create } from "@bufbuild/protobuf";

import {
  CANVAS_SNAP_GRID,
  PIPELINE_NODE_SINK_HANDLE_ID,
  PIPELINE_NODE_SOURCE_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import {
  type PipelineCanvasState,
  type PipelineEdge,
  type PipelineNode,
  type PipelineNodeSink,
  type PipelineNodeSource,
  PipelineNodeType,
} from "@/pages/pipelines/canvas/types";

import { ConnectorKind, IngestionType } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type Pipeline,
  type PipelineEdge as PipelineEdgeProto,
  PipelineSchema,
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

export const mapPipelineToCanvasState = (
  pipeline: Pipeline,
  connections: Connection[],
): Pick<PipelineCanvasState, "nodes" | "edges"> => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));

  // Positions are not persisted (no proto field) - stack deterministically as nodes accumulate
  const nodes: PipelineNode[] = [];
  for (const node of pipeline.nodes) {
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

  const edges: PipelineEdge[] = pipeline.edges.map((edge) => ({
    id: getProtoEdgeKey(edge),
    source: edge.fromNode,
    target: edge.toNode,
    sourceHandle: edge.resource || PIPELINE_NODE_SOURCE_HANDLE_ID,
    targetHandle: PIPELINE_NODE_SINK_HANDLE_ID,
  }));

  return { nodes, edges };
};

export const mapCanvasStateToPipeline = (
  state: PipelineCanvasState,
  basePipeline: Pipeline,
): Pipeline => {
  const baseNodesById = new Map(basePipeline.nodes.map((node) => [node.id, node]));
  const baseEdgesByKey = new Map(basePipeline.edges.map((edge) => [getProtoEdgeKey(edge), edge]));

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

  return create(PipelineSchema, {
    id: basePipeline.id,
    tenant: basePipeline.tenant,
    name: basePipeline.name,
    version: basePipeline.version,
    nodes,
    edges,
  });
};

export const hasPipelineGraphChanges = (
  state: PipelineCanvasState,
  pipeline: Pipeline,
): boolean => {
  const canvasNodes = state.nodes
    .filter(isConnectionNode)
    .map((node) => `${node.id}|${getConnectorKind(node)}|${node.data.connectionId}`)
    .sort();
  const pipelineNodes = pipeline.nodes
    .map(
      (node) =>
        `${node.id}|${node.kind === ConnectorKind.SINK ? ConnectorKind.SINK : ConnectorKind.SOURCE}|${node.connectionId}`,
    )
    .sort();

  const canvasEdges = state.edges.map(getCanvasEdgeKey).sort();
  const pipelineEdges = pipeline.edges.map(getProtoEdgeKey).sort();

  return (
    canvasNodes.join(",") !== pipelineNodes.join(",") ||
    canvasEdges.join(",") !== pipelineEdges.join(",")
  );
};
