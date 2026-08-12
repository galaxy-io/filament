import { useEffect, useLayoutEffect, useRef } from "react";

import { useNodeId, useReactFlow, useUpdateNodeInternals } from "@xyflow/react";

import {
  removePipelineCanvasNodeMeasurements,
  setPipelineCanvasNodeMeasurements,
} from "@/pages/pipelines/canvas/nodes/utils";

export const usePipelineCanvasNodeIslandMeasurements = (hiddenHandleIds: string[]) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const { getZoom } = useReactFlow();
  const listRef = useRef<HTMLDivElement>(null);
  const badgeRef = useRef<HTMLSpanElement>(null);

  const publishMeasurements = () => {
    const listElement = listRef.current;
    const nodeElement = listElement?.closest(".react-flow__node");
    const zoom = getZoom();
    if (!nodeId || !listElement || !nodeElement || zoom <= 0) return;

    const nodeRect = nodeElement.getBoundingClientRect();
    const listRect = listElement.getBoundingClientRect();
    const badgeRect = badgeRef.current?.getBoundingClientRect() ?? null;
    // Whole pixels only: subpixel rects at fractional zoom would republish every render.
    const toNodeX = (clientX: number) => Math.round((clientX - nodeRect.left) / zoom);
    const toNodeY = (clientY: number) => Math.round((clientY - nodeRect.top) / zoom);

    setPipelineCanvasNodeMeasurements(nodeId, {
      listTop: toNodeY(listRect.top),
      listBottom: toNodeY(listRect.bottom),
      badgeAnchorX: badgeRect ? toNodeX(badgeRect.right) : null,
      badgeAnchorY: badgeRect ? toNodeY(badgeRect.top + badgeRect.height / 2) : null,
      hiddenHandleIds,
    });
  };

  const syncMeasurements = () => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
    publishMeasurements();
  };

  // No dependency array: list height and badge presence track content, not any cheap dep.
  // The store's equality bail-out drops the publishes this makes redundant.
  useLayoutEffect(publishMeasurements);

  useEffect(
    () => () => {
      if (nodeId) {
        removePipelineCanvasNodeMeasurements(nodeId);
      }
    },
    [nodeId],
  );

  return { listRef, badgeRef, syncMeasurements };
};
