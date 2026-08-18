import type { JsonValue } from "@bufbuild/protobuf";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
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
import {
  type CanvasEdge,
  type CanvasNode,
  isConnectionNode,
  type PipelineCanvasEdgeData,
} from "@/pages/pipelines/canvas/types";

export const isPipelineRunnable = (version: PipelineVersion | undefined): boolean =>
  (version?.graph?.nodes ?? []).some((node) => node.kind === ConnectorKind.SOURCE) &&
  (version?.graph?.nodes ?? []).some((node) => node.kind === ConnectorKind.SINK) &&
  (version?.graph?.edges ?? []).length > 0;

const canonicalize = (value: JsonValue): JsonValue => {
  if (Array.isArray(value)) return value.map(canonicalize);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, canonicalize(value[key] ?? null)]),
    );
  }
  return value;
};

const serializeNodeConfig = (config: PipelineNode["config"]): string =>
  config && Object.keys(config).length > 0 ? JSON.stringify(canonicalize(config)) : "";

const serializeEdgeConfig = ({ readMode, writeMode, cursors }: PipelineCanvasEdgeData): string =>
  `${readMode}|${writeMode}|${cursors
    .map((cursor) => `${cursor.resource}:${cursor.field}:${cursor.lookbackSeconds}`)
    .sort()
    .join(";")}`;

export const hasPipelineGraphChanges = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  version: PipelineVersion | undefined,
): boolean => {
  const versionNodesById = new Map((version?.graph?.nodes ?? []).map((node) => [node.id, node]));
  const canvasNodes = state.nodes
    .filter(isConnectionNode)
    .map(
      (node) =>
        `${node.id}|${PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type]}|${node.data.connectionId}|${serializeNodeConfig(node.data.config ?? versionNodesById.get(node.id)?.config)}`,
    )
    .sort();
  const pipelineNodes = (version?.graph?.nodes ?? [])
    .map(
      (node) =>
        `${node.id}|${CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP[node.kind]}|${node.connectionId}|${serializeNodeConfig(node.config)}`,
    )
    .sort();

  const versionEdgesByKey = new Map(
    (version?.graph?.edges ?? []).map((edge) => [getProtoEdgeKey(edge), edge]),
  );
  const canvasEdges = state.edges
    .map((edge) => {
      const key = getCanvasEdgeKey(edge);
      return `${key}|${serializeEdgeConfig(getCanvasEdgeConfig(edge, versionEdgesByKey.get(key)))}`;
    })
    .sort();
  const pipelineEdges = (version?.graph?.edges ?? [])
    .map((edge) => `${getProtoEdgeKey(edge)}|${serializeEdgeConfig(edge)}`)
    .sort();

  return (
    canvasNodes.join(",") !== pipelineNodes.join(",") ||
    canvasEdges.join(",") !== pipelineEdges.join(",")
  );
};
