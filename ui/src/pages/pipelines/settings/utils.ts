import { ConnectorKind, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

export const getPipelineCdcSourceConnections = (
  pipeline: Pipeline,
  connections: Connection[],
): Connection[] => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));
  const cdcConnectionsById = new Map<Connection["id"], Connection>();
  for (const node of pipeline.currentVersion?.graph?.nodes ?? []) {
    if (node.kind !== ConnectorKind.SOURCE) continue;
    const connection = connectionsById.get(node.connectionId);
    if (connection?.replication !== ReplicationMode.CDC) continue;
    cdcConnectionsById.set(connection.id, connection);
  }
  return [...cdcConnectionsById.values()];
};
