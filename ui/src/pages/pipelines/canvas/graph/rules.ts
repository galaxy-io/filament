import type { Connection as CanvasConnection } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import {
  PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
  PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
  PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP,
} from "@/pages/pipelines/canvas/constants";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import {
  type CanvasEdge,
  type CanvasNode,
  isConnectionNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";

export const createNodeFromConnection = (
  connection: Connection,
  position: { x: number; y: number },
): CanvasNode => {
  const data = {
    connectionId: connection.id,
  };

  if (connection.kind === ConnectorKind.SOURCE) {
    return { id: crypto.randomUUID(), type: PipelineCanvasNodeType.SOURCE, position, data };
  }

  return { id: crypto.randomUUID(), type: PipelineCanvasNodeType.SINK, position, data };
};

type CanvasEdgeEndpoints = Pick<CanvasEdge, "source" | "target" | "sourceHandle">;

const getPairEdges = (edges: CanvasEdge[], pair: Pick<CanvasEdge, "source" | "target">) =>
  edges.filter((edge) => edge.source === pair.source && edge.target === pair.target);

const mapEdgesToPairs = (edges: CanvasEdge[]): CanvasEdge[][] => {
  const pairs = new Map<string, CanvasEdge[]>();
  for (const edge of edges) {
    const key = `${edge.source} ${edge.target}`;
    pairs.set(key, [...(pairs.get(key) ?? []), edge]);
  }
  return [...pairs.values()];
};

export const canConnectEdge = (connection: CanvasEdgeEndpoints, edges: CanvasEdge[]): boolean => {
  const pairEdges = getPairEdges(edges, connection);
  if (!pairEdges.length) return true;

  const resource = getCanvasEdgeResource(connection);
  if (!resource) return false;

  return pairEdges.every((edge) => {
    const existing = getCanvasEdgeResource(edge);
    return existing !== "" && existing !== resource;
  });
};

export const getPipelineGraphConflicts = (
  edges: CanvasEdge[],
  connectionByNodeId: Map<CanvasNode["id"], Connection | undefined>,
): string[] => {
  const getNodeLabel = (nodeId: CanvasNode["id"]) => connectionByNodeId.get(nodeId)?.name ?? nodeId;

  return mapEdgesToPairs(edges).flatMap((pairEdges) => {
    const { source, target } = pairEdges[0];
    const resources = pairEdges.map(getCanvasEdgeResource);
    const named = resources.filter(Boolean);
    const route = `${getNodeLabel(source)} → ${getNodeLabel(target)}`;

    const conflicts: string[] = [];
    if (named.length && named.length !== resources.length) {
      conflicts.push(
        `${route} routes all resources and individual resources at the same time. Remove one or the other.`,
      );
    }
    const duplicated = [...new Set(named.filter((name, index) => named.indexOf(name) !== index))];
    if (duplicated.length) {
      conflicts.push(`${route} routes ${duplicated.join(", ")} more than once.`);
    }
    return conflicts;
  });
};

export const canAddSourceNode = (nodes: CanvasNode[]): boolean =>
  !nodes.some((node) => node.type === PipelineCanvasNodeType.SOURCE);

export const getAutoConnections = (node: CanvasNode, nodes: CanvasNode[]): CanvasConnection[] => {
  if (!isConnectionNode(node) || nodes.some((existing) => existing.type === node.type)) {
    return [];
  }

  const isSource = node.type === PipelineCanvasNodeType.SOURCE;
  const counterpartType = PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP[node.type];

  return nodes
    .filter((counterpart) => counterpart.type === counterpartType)
    .map((counterpart) => ({
      source: isSource ? node.id : counterpart.id,
      sourceHandle: PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
      target: isSource ? counterpart.id : node.id,
      targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
    }));
};
