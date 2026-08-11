import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import {
  type CanvasNode,
  PipelineCanvasNodeType,
  type PipelineCanvasSinkNode,
  type PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";
import type { PipelineFlowConnection } from "@/pages/pipelines/components/flow/PipelineFlow";

export interface PipelineFlowEndpoints {
  source?: PipelineFlowConnection;
  sinks: PipelineFlowConnection[];
}

export const mapConnectionIdToFlowConnection = (
  connectionId: Connection["id"],
  connectionsById: Map<Connection["id"], Connection>,
): PipelineFlowConnection => {
  const connection = connectionsById.get(connectionId);
  return {
    connectionId,
    connector: connection?.connector ?? "",
    isDeleted: !!connection?.deletedAt,
  };
};

export const mapVersionNodesToFlowEndpoints = (
  nodes: PipelineVersion["nodes"],
  connections: Connection[],
): PipelineFlowEndpoints => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));

  const sourceNode = nodes.find((node) => node.kind === ConnectorKind.SOURCE);
  return {
    source: sourceNode
      ? mapConnectionIdToFlowConnection(sourceNode.connectionId, connectionsById)
      : undefined,
    sinks: nodes
      .filter((node) => node.kind === ConnectorKind.SINK)
      .map((node) => mapConnectionIdToFlowConnection(node.connectionId, connectionsById)),
  };
};

export const mapCanvasNodesToFlowEndpoints = (
  nodes: CanvasNode[],
  connections: Connection[],
): PipelineFlowEndpoints => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));

  const sourceNode = nodes.find(
    (node): node is PipelineCanvasSourceNode => node.type === PipelineCanvasNodeType.SOURCE,
  );
  return {
    source: sourceNode
      ? mapConnectionIdToFlowConnection(sourceNode.data.connectionId, connectionsById)
      : undefined,
    sinks: nodes
      .filter((node): node is PipelineCanvasSinkNode => node.type === PipelineCanvasNodeType.SINK)
      .map((node) => mapConnectionIdToFlowConnection(node.data.connectionId, connectionsById)),
  };
};
