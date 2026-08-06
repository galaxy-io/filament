import type { Theme } from "@galaxy-io/dls/theme/types";

import type { IngestionType } from "@/gen/ingestion/v1/common_pb";

import {
  CURSOR_BEARING_INGESTION_TYPES,
  PIPELINE_CANVAS_DEFAULT_EDGE_DATA,
  PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import type {
  CanvasEdge,
  CanvasNode,
  PipelineCanvasEdgeData,
} from "@/pages/pipelines/canvas/types";

export const getCanvasEdgeResource = (edge: CanvasEdge) =>
  edge.sourceHandle && edge.sourceHandle !== PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID
    ? edge.sourceHandle
    : "";

export const getPipelineCanvasEdgeData = (edge: CanvasEdge): PipelineCanvasEdgeData =>
  edge.data ?? PIPELINE_CANVAS_DEFAULT_EDGE_DATA;

export const isIncrementalIngestionType = (ingestionType: IngestionType): boolean =>
  CURSOR_BEARING_INGESTION_TYPES.has(ingestionType);

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
      style: {
        stroke: isHighlighted ? theme.color.background.galaxy : theme.color.border.primary,
        strokeWidth: edge.selected ? 3 : 2,
      },
    };
  });
};
