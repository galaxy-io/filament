import { useMemo } from "react";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { type CanvasNode, isConnectionNode } from "@/pages/pipelines/canvas/types";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";

export const usePipelineCanvasConnections = () => {
  const state = usePipelineCanvasState();
  const { data } = useSuspenseListConnectionsQuery();

  return useMemo(() => {
    const connectionsById = new Map(
      data.connections.map((connection) => [connection.id, connection]),
    );

    return new Map<CanvasNode["id"], Connection | undefined>(
      state.nodes
        .filter(isConnectionNode)
        .map((node) => [node.id, connectionsById.get(node.data.connectionId)]),
    );
  }, [state.nodes, data.connections]);
};
