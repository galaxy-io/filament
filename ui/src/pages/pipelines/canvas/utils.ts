import type { FitViewOptions } from "@xyflow/react";

import type { Theme } from "@galaxy-io/dls/theme/types";

import type { PipelineEdge } from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_CANVAS_EDGE_Z_INDEX,
  PIPELINE_CANVAS_FIT_INSET_LEFT,
  PIPELINE_CANVAS_FIT_INSET_Y,
  PIPELINE_CANVAS_FIT_MAX_ZOOM,
} from "@/pages/pipelines/canvas/constants";
import {
  PIPELINE_CANVAS_PANEL_COLLAPSED_WIDTH,
  PIPELINE_CANVAS_PANEL_INSET,
  PIPELINE_CANVAS_PANEL_WIDTH,
} from "@/pages/pipelines/canvas/panel/constants";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export const getPipelineCanvasFitPadding = (
  showPanel: boolean,
): NonNullable<FitViewOptions["padding"]> => ({
  top: `${PIPELINE_CANVAS_FIT_INSET_Y}px`,
  bottom: `${PIPELINE_CANVAS_FIT_INSET_Y}px`,
  left: `${PIPELINE_CANVAS_FIT_INSET_LEFT}px`,
  right: `${
    PIPELINE_CANVAS_PANEL_INSET * 2 +
    (showPanel ? PIPELINE_CANVAS_PANEL_WIDTH : PIPELINE_CANVAS_PANEL_COLLAPSED_WIDTH)
  }px`,
});

export const getPipelineCanvasFitViewOptions = (showPanel: boolean): FitViewOptions => ({
  padding: getPipelineCanvasFitPadding(showPanel),
  maxZoom: PIPELINE_CANVAS_FIT_MAX_ZOOM,
});

export const mapElementsToSelected = <T extends { id: string; selected?: boolean }>(
  elements: T[],
  selectedId: string | undefined,
): T[] =>
  elements.map((element) => {
    const selected = element.id === selectedId;
    return element.selected === selected ? element : { ...element, selected };
  });

export const getCanvasEdgeResourceLabel = (
  resource: PipelineEdge["resource"],
  coveredCount: number,
): { label: string; isNamedResource: boolean } => {
  if (resource) return { label: resource, isNamedResource: true };
  return {
    label: coveredCount ? `${coveredCount} resources` : "All resources",
    isNamedResource: false,
  };
};

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
