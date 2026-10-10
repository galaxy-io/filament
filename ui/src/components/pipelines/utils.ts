import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import type { PipelineFlowConnection } from "@/components/pipelines/PipelineFlow";

const DELETED_NAME_SUFFIX = /__deleted__\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;

export const stripDeletedName = (name: string) => name.replace(DELETED_NAME_SUFFIX, "");

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
  nodes: NonNullable<PipelineVersion["graph"]>["nodes"],
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

export const formatPipelineName = (pipeline: Pipeline, includeDeleted = false): string => {
  const name = includeDeleted ? pipeline.name : stripDeletedName(pipeline.name);
  if (name) {
    return name.replace(/->/g, "→");
  }
  return pipeline.id;
};

export const isPipelineNameMatch = (typed: string, name: string): boolean =>
  typed.trim().replace(/->/g, "→") === name.trim();
