import { create } from "@bufbuild/protobuf";

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
  getCanvasEdgeIngestionType,
  isConnectionNode,
} from "@/pages/pipelines/canvas/types";

const getCanvasEdgeResource = (edge: CanvasEdge) =>
  edge.sourceHandle && edge.sourceHandle !== PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID
    ? edge.sourceHandle
    : "";

export const normalizeIngestionType = (type: IngestionType): IngestionType =>
  type === IngestionType.UNSPECIFIED ? IngestionType.SNAPSHOT_REPLACE : type;

export const getProtoEdgeKey = (edge: PipelineEdgeProto) =>
  `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

export const getCanvasEdgeKey = (edge: CanvasEdge) =>
  `${edge.source}|${getCanvasEdgeResource(edge)}|${edge.target}`;

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
    data: { ingestionType: normalizeIngestionType(edge.ingestionType) },
  }));

  return { nodes, edges };
};

export const mapCanvasStateToVersionRequest = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
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
      kind: PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type],
      connectionId: node.data.connectionId,
      config: node.data.config ?? baseNode?.config,
      secretRefs: baseNode?.secretRefs ?? {},
    };
  });

  const edges = state.edges.map((edge) => {
    const baseEdge = baseEdgesByKey.get(getCanvasEdgeKey(edge));
    return {
      fromNode: edge.source,
      resource: getCanvasEdgeResource(edge),
      toNode: edge.target,
      ingestionType: getCanvasEdgeIngestionType(edge),
      selector: baseEdge?.selector ?? "",
    };
  });

  return create(CreatePipelineVersionRequestSchema, {
    pipelineId,
    nodes,
    edges,
  });
};
