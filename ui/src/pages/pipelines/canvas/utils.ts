import type { Theme } from "@galaxy-io/dls/theme/types";

import { PIPELINE_CANVAS_EDGE_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export const mapEdgesToStyledEdges = (
  edges: CanvasEdge[],
  nodes: CanvasNode[],
  theme: Theme,
): CanvasEdge[] => {
  const selectedNodeIds = new Set(nodes.filter((node) => node.selected).map((node) => node.id));

  return edges.map((edge) => {
    const isConnectedToSelected =
      selectedNodeIds.has(edge.source) || selectedNodeIds.has(edge.target);
    const isHighlighted = edge.selected || isConnectedToSelected;

    return {
      ...edge,
      zIndex: PIPELINE_CANVAS_EDGE_Z_INDEX,
      style: {
        stroke: isHighlighted ? theme.color.background.galaxy : theme.color.border.primary,
        strokeWidth: edge.selected ? 3 : 2,
      },
    };
  });
};
