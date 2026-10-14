import { useMemo } from "react";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasNode } from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";

import { useGetConnectionQueries } from "@/api/queries/connections";

export const usePipelineCanvasConnections = () => {
  const state = usePipelineCanvasState();

  const connectionNodes = useMemo(() => state.nodes.filter(isConnectionNode), [state.nodes]);
  const connectionIds = useMemo(
    () => [...new Set(connectionNodes.map((node) => node.data.connectionId))],
    [connectionNodes],
  );
  const connections = useGetConnectionQueries(connectionIds);

  return useMemo(() => {
    const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));
    return new Map<CanvasNode["id"], Connection | undefined>(
      connectionNodes.map((node) => [node.id, connectionsById.get(node.data.connectionId)]),
    );
  }, [connectionNodes, connections]);
};
