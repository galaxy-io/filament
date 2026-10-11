import type { JsonValue } from "@bufbuild/protobuf";

import type { PipelineNode, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import {
  CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP,
  PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP,
} from "@/pages/pipelines/canvas/constants";
import {
  getCanvasEdgeConfig,
  getCanvasEdgeKey,
  getProtoEdgeKey,
} from "@/pages/pipelines/canvas/graph/serialize";
import type {
  CanvasEdge,
  CanvasNode,
  PipelineCanvasEdgeData,
} from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";

const getCanonicalJson = (value: JsonValue): JsonValue => {
  if (Array.isArray(value)) return value.map(getCanonicalJson);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, getCanonicalJson(value[key] ?? null)]),
    );
  }
  return value;
};

const getNodeConfigKey = (config: PipelineNode["config"]): string =>
  config && Object.keys(config).length > 0 ? JSON.stringify(getCanonicalJson(config)) : "";

const getEdgeConfigKey = ({
  readMode,
  writeMode,
  cursors,
  transform,
  destinationResource,
}: PipelineCanvasEdgeData): string =>
  `${JSON.stringify(destinationResource ?? "")}|${readMode}|${writeMode}|${cursors
    .map((cursor) => `${cursor.resource}:${cursor.field}:${cursor.lookbackSeconds}`)
    .sort()
    .join(";")}|${transform ? JSON.stringify(getCanonicalJson(transform)) : ""}`;

export const hasPipelineGraphChanges = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  version: PipelineVersion | undefined,
): boolean => {
  const versionNodesById = new Map((version?.graph?.nodes ?? []).map((node) => [node.id, node]));
  const canvasNodes = state.nodes
    .filter(isConnectionNode)
    .map(
      (node) =>
        `${node.id}|${PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type]}|${node.data.connectionId}|${getNodeConfigKey(node.data.config ?? versionNodesById.get(node.id)?.config)}`,
    )
    .sort();
  const pipelineNodes = (version?.graph?.nodes ?? [])
    .map(
      (node) =>
        `${node.id}|${CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP[node.kind]}|${node.connectionId}|${getNodeConfigKey(node.config)}`,
    )
    .sort();

  const versionEdgesByKey = new Map(
    (version?.graph?.edges ?? []).map((edge) => [getProtoEdgeKey(edge), edge]),
  );
  const canvasEdges = state.edges
    .map((edge) => {
      const key = getCanvasEdgeKey(edge);
      return `${key}|${getEdgeConfigKey(getCanvasEdgeConfig(edge, versionEdgesByKey.get(key)))}`;
    })
    .sort();
  const pipelineEdges = (version?.graph?.edges ?? [])
    .map((edge) => `${getProtoEdgeKey(edge)}|${getEdgeConfigKey(edge)}`)
    .sort();

  return (
    canvasNodes.join(",") !== pipelineNodes.join(",") ||
    canvasEdges.join(",") !== pipelineEdges.join(",")
  );
};
