import { useMemo } from "react";

import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasActiveRuns } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasActiveRuns";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { isConnectionNode, isPipelineCanvasRouteRunning } from "@/pages/pipelines/canvas/utils";

export const usePipelineCanvasRunningEdgeIds = (
  edges: CanvasEdge[],
  nodes: CanvasNode[],
): Set<CanvasEdge["id"]> => {
  const runs = usePipelineCanvasActiveRuns();

  return useMemo(() => {
    const connectionByNodeId = new Map(
      nodes.filter(isConnectionNode).map((node) => [node.id, node.data.connectionId]),
    );
    return new Set(
      edges
        .filter((edge) =>
          isPipelineCanvasRouteRunning(
            runs,
            connectionByNodeId.get(edge.source),
            connectionByNodeId.get(edge.target),
            getCanvasEdgeResource(edge),
          ),
        )
        .map((edge) => edge.id),
    );
  }, [edges, nodes, runs]);
};
