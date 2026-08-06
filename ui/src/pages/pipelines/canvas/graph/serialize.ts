import { create } from "@bufbuild/protobuf";

import {
  type ValidatePipelineRequest,
  ValidatePipelineRequestSchema,
} from "@/gen/ingestion/v1/capabilities_pb";
import { IngestionType } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreatePipelineVersionRequest,
  CreatePipelineVersionRequestSchema,
  type PipelineEdge as PipelineEdgeProto,
  type PipelineVersion,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  CONNECTOR_KIND_TO_NODE_TYPE_MAP,
  CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP,
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
  PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
  PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP,
} from "@/pages/pipelines/canvas/constants";
import { getNextNodePosition } from "@/pages/pipelines/canvas/graph/layout";
import {
  type CanvasEdge,
  type CanvasNode,
  isConnectionNode,
  type PipelineCanvasEdgeData,
  type PipelineCanvasWireEdge,
} from "@/pages/pipelines/canvas/types";
import {
  getCanvasEdgeResource,
  getPipelineCanvasEdgeData,
  isIncrementalIngestionType,
} from "@/pages/pipelines/canvas/utils";

export const getProtoEdgeKey = (
  edge: Pick<PipelineCanvasWireEdge, "fromNode" | "resource" | "toNode">,
) => `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

export const getCanvasEdgeKey = (edge: CanvasEdge) =>
  `${edge.source}|${getCanvasEdgeResource(edge)}|${edge.target}`;

export const normalizeIngestionType = (ingestionType: IngestionType): IngestionType =>
  ingestionType === IngestionType.UNSPECIFIED ? IngestionType.SNAPSHOT_REPLACE : ingestionType;

const mapProtoEdgeToEdgeData = (edge: PipelineEdgeProto): PipelineCanvasEdgeData => ({
  ingestionType: normalizeIngestionType(edge.ingestionType),
  selector: edge.selector,
  cursors: edge.cursors,
});

export const mapCanvasEdgeToProtoEdge = (edge: CanvasEdge): PipelineCanvasWireEdge => {
  const data = getPipelineCanvasEdgeData(edge);
  return {
    fromNode: edge.source,
    resource: getCanvasEdgeResource(edge),
    toNode: edge.target,
    ingestionType: data.ingestionType,
    selector: data.selector,
    cursors: isIncrementalIngestionType(data.ingestionType) ? data.cursors : [],
  };
};

export const mapPipelineVersionToCanvasState = (
  version: PipelineVersion | undefined,
  connections: Connection[],
): { nodes: CanvasNode[]; edges: CanvasEdge[] } => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));

  const nodes: CanvasNode[] = [];
  for (const node of version?.nodes ?? []) {
    const connection = connectionsById.get(node.connectionId);
    const kind = CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP[node.kind];
    const type = CONNECTOR_KIND_TO_NODE_TYPE_MAP[kind];

    nodes.push({
      id: node.id,
      type,
      position: getNextNodePosition(kind, nodes),
      data: {
        label: connection?.name ?? node.connectionId,
        connector: connection?.connector ?? "",
        connectionId: node.connectionId,
        config: node.config,
      },
    });
  }

  const edges: CanvasEdge[] = (version?.edges ?? []).map((edge) => ({
    id: getProtoEdgeKey(edge),
    type: PIPELINE_CANVAS_EDGE_TYPE,
    source: edge.fromNode,
    target: edge.toNode,
    sourceHandle: edge.resource || PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
    targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
    data: mapProtoEdgeToEdgeData(edge),
  }));

  return { nodes, edges };
};

// buildPipelineGraph is the single wire shape both saving and validation use,
// so a graph can never validate as something other than what would be saved.
export const buildPipelineGraph = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  baseVersion: PipelineVersion | undefined,
) => {
  const baseNodesById = new Map((baseVersion?.nodes ?? []).map((node) => [node.id, node]));

  const nodes = state.nodes.filter(isConnectionNode).map((node) => {
    const baseNode = baseNodesById.get(node.id);
    return {
      id: node.id,
      kind: PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type],
      connectionId: node.data.connectionId,
      config: node.data.config ?? baseNode?.config,
      secretRefs: baseNode?.secretRefs ?? {},
    };
  });

  const nodeIds = new Set(nodes.map((node) => node.id));
  const edges = state.edges
    .filter((edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target))
    .map(mapCanvasEdgeToProtoEdge);

  return { nodes, edges };
};

export const mapCanvasStateToVersionRequest = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  pipelineId: string,
  baseVersion: PipelineVersion | undefined,
): CreatePipelineVersionRequest =>
  create(CreatePipelineVersionRequestSchema, {
    pipelineId,
    ...buildPipelineGraph(state, baseVersion),
  });

export const buildValidatePipelineRequest = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  baseVersion: PipelineVersion | undefined,
): ValidatePipelineRequest =>
  create(ValidatePipelineRequestSchema, buildPipelineGraph(state, baseVersion));
