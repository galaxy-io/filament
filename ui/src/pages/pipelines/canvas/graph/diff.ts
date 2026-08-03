import type { JsonValue } from "@bufbuild/protobuf";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import {
  CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP,
  PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP,
} from "@/pages/pipelines/canvas/constants";
import {
  getCanvasEdgeKey,
  getProtoEdgeKey,
  normalizeIngestionType,
} from "@/pages/pipelines/canvas/graph/serialize";
import {
  type CanvasEdge,
  type CanvasNode,
  getCanvasEdgeIngestionType,
  isConnectionNode,
} from "@/pages/pipelines/canvas/types";

export const isPipelineRunnable = (version: PipelineVersion | undefined): boolean =>
  (version?.nodes ?? []).some((node) => node.kind === ConnectorKind.SOURCE) &&
  (version?.nodes ?? []).some((node) => node.kind === ConnectorKind.SINK) &&
  (version?.edges ?? []).length > 0;

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

const serializeNodeConfig = (config: Record<string, JsonValue> | undefined): string =>
  config && Object.keys(config).length > 0 ? JSON.stringify(canonicalize(config)) : "";

export const hasPipelineGraphChanges = (
  state: { nodes: CanvasNode[]; edges: CanvasEdge[] },
  version: PipelineVersion | undefined,
): boolean => {
  const versionNodesById = new Map((version?.nodes ?? []).map((node) => [node.id, node]));
  const canvasNodes = state.nodes
    .filter(isConnectionNode)
    .map(
      (node) =>
        `${node.id}|${PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type]}|${node.data.connectionId}|${serializeNodeConfig(node.data.config ?? versionNodesById.get(node.id)?.config)}`,
    )
    .sort();
  const pipelineNodes = (version?.nodes ?? [])
    .map(
      (node) =>
        `${node.id}|${CONNECTOR_KIND_TO_NORMALIZED_KIND_MAP[node.kind]}|${node.connectionId}|${serializeNodeConfig(node.config)}`,
    )
    .sort();

  const canvasEdges = state.edges
    .map((edge) => `${getCanvasEdgeKey(edge)}|${getCanvasEdgeIngestionType(edge)}`)
    .sort();
  const pipelineEdges = (version?.edges ?? [])
    .map((edge) => `${getProtoEdgeKey(edge)}|${normalizeIngestionType(edge.ingestionType)}`)
    .sort();

  return (
    canvasNodes.join(",") !== pipelineNodes.join(",") ||
    canvasEdges.join(",") !== pipelineEdges.join(",")
  );
};
