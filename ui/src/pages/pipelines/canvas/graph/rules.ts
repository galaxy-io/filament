import type { Connection as CanvasConnection } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

import {
  PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
  PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
  PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP,
} from "@/pages/pipelines/canvas/constants";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import {
  type CanvasEdge,
  type CanvasNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";

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

export const canConnectEdge = (connection: CanvasEdgeEndpoints, edges: CanvasEdge[]): boolean => {
  const resource = getCanvasEdgeResource(connection);
  return (
    resource !== "" &&
    !getPairEdges(edges, connection).some((edge) => getCanvasEdgeResource(edge) === resource)
  );
};

export const expandConnection = <T extends CanvasEdgeEndpoints>(
  connection: T,
  resources: Resource["name"][],
): T[] =>
  connection.sourceHandle === PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID
    ? resources.map((resource) => ({ ...connection, sourceHandle: resource }))
    : [connection];

export const canAddSourceNode = (nodes: CanvasNode[]): boolean =>
  !nodes.some((node) => node.type === PipelineCanvasNodeType.SOURCE);

export const getAutoConnections = (
  node: CanvasNode,
  nodes: CanvasNode[],
  resources: Resource["name"][],
): CanvasConnection[] => {
  if (!isConnectionNode(node) || nodes.some((existing) => existing.type === node.type)) {
    return [];
  }

  const isSource = node.type === PipelineCanvasNodeType.SOURCE;
  const counterpartType = PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP[node.type];

  return nodes
    .filter((counterpart) => counterpart.type === counterpartType)
    .flatMap((counterpart) =>
      expandConnection(
        {
          source: isSource ? node.id : counterpart.id,
          sourceHandle: PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
          target: isSource ? counterpart.id : node.id,
          targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
        },
        resources,
      ),
    );
};
