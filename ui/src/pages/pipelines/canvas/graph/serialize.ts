import { create } from "@bufbuild/protobuf";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import {
  type CreatePipelineVersionRequest,
  CreatePipelineVersionRequestSchema,
  type Pipeline,
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
} from "@/pages/pipelines/canvas/types";

export const getCanvasEdgeResource = (
  edge: Pick<CanvasEdge, "sourceHandle">,
): PipelineEdgeProto["resource"] =>
  edge.sourceHandle && edge.sourceHandle !== PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID
    ? edge.sourceHandle
    : "";

export const getCanvasEdgeConfig = (
  edge: Pick<CanvasEdge, "data">,
  baseEdge: PipelineEdgeProto | undefined,
): PipelineCanvasEdgeData => ({
  readMode: edge.data?.readMode ?? baseEdge?.readMode ?? ReadMode.UNSPECIFIED,
  writeMode: edge.data?.writeMode ?? baseEdge?.writeMode ?? WriteMode.UNSPECIFIED,
  cursors: edge.data?.cursors ?? baseEdge?.cursors ?? [],
});

export const getProtoEdgeKey = (edge: PipelineEdgeProto) =>
  `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

export const getCanvasEdgeKey = (edge: CanvasEdge) =>
  `${edge.source}|${getCanvasEdgeResource(edge)}|${edge.target}`;

export const mapPipelineVersionToCanvasState = (
  version: PipelineVersion | undefined,
): { nodes: CanvasNode[]; edges: CanvasEdge[] } => {
  const nodes: CanvasNode[] = [];
  for (const node of version?.graph?.nodes ?? []) {
    const kind = CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP[node.kind];
    const type = CONNECTOR_KIND_TO_NODE_TYPE_MAP[kind];

    nodes.push({
      id: node.id,
      type,
      position: getNextNodePosition(kind, nodes),
      data: {
        connectionId: node.connectionId,
        config: node.config,
      },
    });
  }

  const seenEdgeKeys = new Set<string>();
  const edges: CanvasEdge[] = (version?.graph?.edges ?? [])
    .filter((edge) => {
      const key = getProtoEdgeKey(edge);
      if (seenEdgeKeys.has(key)) return false;
      seenEdgeKeys.add(key);
      return true;
    })
    .map((edge) => ({
      id: getProtoEdgeKey(edge),
      type: PIPELINE_CANVAS_EDGE_TYPE,
      source: edge.fromNode,
      target: edge.toNode,
      sourceHandle: edge.resource || PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
      targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
      data: getCanvasEdgeConfig({}, edge),
    }));

  return { nodes, edges };
};

export const mapCanvasStateToVersionRequest = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  pipelineId: Pipeline["id"],
  baseVersion: PipelineVersion | undefined,
): CreatePipelineVersionRequest => {
  const baseNodesById = new Map((baseVersion?.graph?.nodes ?? []).map((node) => [node.id, node]));
  const baseEdgesByKey = new Map(
    (baseVersion?.graph?.edges ?? []).map((edge) => [getProtoEdgeKey(edge), edge]),
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
      selector: baseEdge?.selector ?? "",
      ...getCanvasEdgeConfig(edge, baseEdge),
    };
  });

  return create(CreatePipelineVersionRequestSchema, {
    pipelineId,
    graph: { nodes, edges },
  });
};
