import type { InternalNode, Node, XYPosition } from "@xyflow/react";

import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";
import type { PipelineCanvasNodeIslandMeasurements } from "@/pages/pipelines/canvas/nodes/utils";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

export const getPipelineCanvasEdgeAnchor = ({
  sourceNode,
  sourceHandle,
  sourceX,
  sourceY,
  measurements,
}: {
  sourceNode: InternalNode<Node> | undefined;
  sourceHandle: CanvasEdge["sourceHandle"];
  sourceX: number;
  sourceY: number;
  measurements: PipelineCanvasNodeIslandMeasurements | undefined;
}): XYPosition => {
  const { positionAbsolute } = sourceNode?.internals ?? {};
  const { width, height } = sourceNode?.measured ?? {};
  if (!positionAbsolute || !width || !height) return { x: sourceX, y: sourceY };

  const nodeTop = positionAbsolute.y;
  const anchor = {
    x: Math.min(sourceX, positionAbsolute.x + width),
    y: clamp(
      sourceY,
      nodeTop + PIPELINE_CANVAS_NODE_PADDING,
      nodeTop + height - PIPELINE_CANVAS_NODE_PADDING,
    ),
  };

  const resource = getCanvasEdgeResource({ sourceHandle });
  const { badgeAnchorX, badgeAnchorY } = measurements ?? {};
  if (!resource || !measurements || badgeAnchorX == null || badgeAnchorY == null) return anchor;

  // Hidden rows are display:none, so their measured sourceY is garbage — test that first.
  const isRowVisible =
    !measurements.hiddenHandleIds.includes(resource) &&
    sourceY >= nodeTop + measurements.listTop &&
    sourceY <= nodeTop + measurements.listBottom;
  if (isRowVisible) return anchor;

  return { x: positionAbsolute.x + badgeAnchorX, y: nodeTop + badgeAnchorY };
};
