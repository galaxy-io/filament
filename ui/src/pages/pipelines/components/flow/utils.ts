import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import {
  type CanvasNode,
  PipelineNodeType,
  type PipelineSinkNode,
  type PipelineSourceNode,
} from "@/pages/pipelines/canvas/types";
import type { PipelineFlowConnection } from "@/pages/pipelines/components/flow/PipelineFlow";

export interface PipelineFlowEndpoints {
  source?: PipelineFlowConnection;
  sinks: PipelineFlowConnection[];
}

export const mapVersionNodesToFlowEndpoints = (
  nodes: PipelineVersion["nodes"],
  connections: Connection[],
): PipelineFlowEndpoints => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));
  const toFlowConnection = (connectionId: string): PipelineFlowConnection => ({
    connectionId,
    connector: connectionsById.get(connectionId)?.connector ?? connectionId,
  });

  const sourceNode = nodes.find((node) => node.kind === ConnectorKind.SOURCE);
  return {
    source: sourceNode ? toFlowConnection(sourceNode.connectionId) : undefined,
    sinks: nodes
      .filter((node) => node.kind === ConnectorKind.SINK)
      .map((node) => toFlowConnection(node.connectionId)),
  };
};

export const mapCanvasNodesToFlowEndpoints = (nodes: CanvasNode[]): PipelineFlowEndpoints => {
  const sourceNode = nodes.find(
    (node): node is PipelineSourceNode => node.type === PipelineNodeType.SOURCE,
  );
  return {
    source: sourceNode
      ? { connectionId: sourceNode.data.connectionId, connector: sourceNode.data.connector }
      : undefined,
    sinks: nodes
      .filter((node): node is PipelineSinkNode => node.type === PipelineNodeType.SINK)
      .map((node) => ({ connectionId: node.data.connectionId, connector: node.data.connector })),
  };
};
