import { useMemo } from "react";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  type CanvasNode,
  isConnectionNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";

export interface PipelineCanvasRoutesSink {
  nodeId: CanvasNode["id"];
  label: string;
  connection: Connection | undefined;
}

export const usePipelineCanvasRoutesSinks = (): PipelineCanvasRoutesSink[] => {
  const { nodes } = usePipelineCanvasState();
  const connectionByNodeId = usePipelineCanvasConnections();

  return useMemo(
    () =>
      nodes
        .filter(isConnectionNode)
        .filter((node) => node.type === PipelineCanvasNodeType.SINK)
        .map((node) => {
          const connection = connectionByNodeId.get(node.id);
          return {
            nodeId: node.id,
            label: connection?.name ?? node.data.connectionId,
            connection,
          };
        }),
    [nodes, connectionByNodeId],
  );
};
