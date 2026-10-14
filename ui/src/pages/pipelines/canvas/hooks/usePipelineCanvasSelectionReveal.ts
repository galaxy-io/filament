import { useEffect } from "react";

import { getViewportForBounds, useReactFlow, useStore } from "@xyflow/react";

import {
  PIPELINE_CANVAS_FIT_INSET_LEFT,
  PIPELINE_CANVAS_FIT_INSET_Y,
  PIPELINE_CANVAS_FIT_MAX_ZOOM,
  PIPELINE_CANVAS_FIT_MIN_ZOOM,
  PIPELINE_CANVAS_FIT_PADDING,
  PIPELINE_CANVAS_REVEAL_DURATION,
  PIPELINE_CANVAS_REVEAL_ZOOM,
} from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";

export const usePipelineCanvasSelectionReveal = () => {
  const { getViewport, setViewport, getInternalNode, getEdge, getNodes, getNodesBounds } =
    useReactFlow();
  const width = useStore((s) => s.width);
  const height = useStore((s) => s.height);
  const { selectedNodeId, selectedResourceId, showPanel } = usePipelineCanvasSelection();

  useEffect(() => {
    if (!width || !height) return;

    const edge = selectedResourceId ? getEdge(selectedResourceId) : undefined;
    const selectedIds = selectedNodeId ? [selectedNodeId] : edge ? [edge.source, edge.target] : [];
    const nodeIds =
      showPanel && selectedIds.length ? selectedIds : getNodes().map((node) => node.id);
    if (!nodeIds.length) return;
    if (nodeIds.some((nodeId) => !getInternalNode(nodeId)?.measured.width)) return;

    const bounds = getNodesBounds(nodeIds);

    if (!showPanel || !selectedIds.length) {
      void setViewport(
        getViewportForBounds(
          bounds,
          width,
          height,
          PIPELINE_CANVAS_FIT_MIN_ZOOM,
          PIPELINE_CANVAS_FIT_MAX_ZOOM,
          PIPELINE_CANVAS_FIT_PADDING,
        ),
        { duration: PIPELINE_CANVAS_REVEAL_DURATION },
      );
      return;
    }

    const { zoom } = getViewport();
    const maxFitZoom = Math.min(
      (width - PIPELINE_CANVAS_FIT_INSET_LEFT * 2) / bounds.width,
      (height - PIPELINE_CANVAS_FIT_INSET_Y * 2) / bounds.height,
    );
    const targetZoom = Math.max(
      PIPELINE_CANVAS_FIT_MIN_ZOOM,
      Math.min(Math.max(zoom, PIPELINE_CANVAS_REVEAL_ZOOM), maxFitZoom),
    );

    void setViewport(
      {
        x:
          (PIPELINE_CANVAS_FIT_INSET_LEFT + width) / 2 - (bounds.x + bounds.width / 2) * targetZoom,
        y: height / 2 - (bounds.y + bounds.height / 2) * targetZoom,
        zoom: targetZoom,
      },
      {
        duration: PIPELINE_CANVAS_REVEAL_DURATION,
        interpolate: targetZoom === zoom ? "linear" : "smooth",
      },
    );
  }, [
    selectedNodeId,
    selectedResourceId,
    showPanel,
    width,
    height,
    getEdge,
    getInternalNode,
    getNodes,
    getNodesBounds,
    getViewport,
    setViewport,
  ]);
};
