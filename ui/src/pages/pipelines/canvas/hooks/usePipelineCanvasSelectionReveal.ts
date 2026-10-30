import { useCallback, useEffect, useRef } from "react";

import { getViewportForBounds, useNodesInitialized, useReactFlow, useStore } from "@xyflow/react";

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
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  type CanvasEdge,
  type CanvasNode,
  PipelineCanvasNodeType,
  type PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";
import { getPipelineCanvasRevealBounds } from "@/pages/pipelines/canvas/utils";

import { useIsDiscoveringResources } from "@/api/queries/connectors";

export const usePipelineCanvasSelectionReveal = () => {
  const { getViewport, setViewport, getInternalNode, getEdge, getNodes, getNodesBounds } =
    useReactFlow<CanvasNode, CanvasEdge>();
  const width = useStore((s) => s.width);
  const height = useStore((s) => s.height);
  const { selectedNodeId, selectedResourceId, showPanel } = usePipelineCanvasSelection();
  const { nodes } = usePipelineCanvasState();
  const sourceConnectionId = nodes.find(
    (node): node is PipelineCanvasSourceNode => node.type === PipelineCanvasNodeType.SOURCE,
  )?.data.connectionId;
  const isNodesInitialized = useNodesInitialized();
  const isDiscovering = useIsDiscoveringResources(sourceConnectionId);
  const isDiscoveringRef = useRef(isDiscovering);
  const wasDiscoveringRef = useRef(false);
  const hasRevealedRef = useRef(false);

  const reveal = useCallback(() => {
    if (!width || !height) return;

    const edge = selectedResourceId ? getEdge(selectedResourceId) : undefined;
    const selectedIds = selectedNodeId ? [selectedNodeId] : edge ? [edge.source, edge.target] : [];
    const nodeIds =
      showPanel && selectedIds.length ? selectedIds : getNodes().map((node) => node.id);
    if (!nodeIds.length) return;
    if (nodeIds.some((nodeId) => !getInternalNode(nodeId)?.measured.width)) return;

    const bounds = getPipelineCanvasRevealBounds(
      getNodesBounds(nodeIds),
      getNodes().filter((node) => nodeIds.includes(node.id)),
      isDiscoveringRef.current,
    );
    const duration = hasRevealedRef.current ? PIPELINE_CANVAS_REVEAL_DURATION : 0;
    hasRevealedRef.current = true;

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
        { duration },
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
        duration,
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

  useEffect(() => {
    isDiscoveringRef.current = isDiscovering;
  }, [isDiscovering]);

  useEffect(() => {
    if (hasRevealedRef.current) reveal();
  }, [reveal]);

  useEffect(() => {
    if (!isNodesInitialized) return;
    if (!hasRevealedRef.current) reveal();
    if (isDiscovering) {
      wasDiscoveringRef.current = true;
      return;
    }
    if (!wasDiscoveringRef.current) return;
    wasDiscoveringRef.current = false;
    let frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(reveal);
    });
    return () => cancelAnimationFrame(frame);
  }, [isNodesInitialized, isDiscovering, reveal]);
};
