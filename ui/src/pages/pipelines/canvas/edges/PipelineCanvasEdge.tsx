import { useSyncExternalStore } from "react";

import { BaseEdge, type EdgeProps, getBezierPath, useInternalNode } from "@xyflow/react";

import { PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";
import {
  getPipelineCanvasNodeMeasurements,
  subscribePipelineCanvasNodeMeasurements,
} from "@/pages/pipelines/canvas/nodes/utils";

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

const PipelineCanvasEdge = ({
  id,
  source,
  sourceHandleId,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style,
  markerEnd,
}: EdgeProps) => {
  const sourceNode = useInternalNode(source);
  const sourceMeasurements = useSyncExternalStore(subscribePipelineCanvasNodeMeasurements, () =>
    getPipelineCanvasNodeMeasurements(source),
  );

  let anchorX = sourceX;
  let anchorY = sourceY;

  const { positionAbsolute } = sourceNode?.internals ?? {};
  const { width, height } = sourceNode?.measured ?? {};
  if (positionAbsolute && width && height) {
    const nodeTop = positionAbsolute.y;
    const nodeBottom = nodeTop + height;

    anchorY = clamp(
      sourceY,
      nodeTop + PIPELINE_CANVAS_NODE_PADDING,
      nodeBottom - PIPELINE_CANVAS_NODE_PADDING,
    );

    const isResourceEdge =
      sourceHandleId && sourceHandleId !== PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID;
    if (
      isResourceEdge &&
      sourceMeasurements &&
      sourceMeasurements.badgeAnchorX !== null &&
      sourceMeasurements.badgeAnchorY !== null
    ) {
      const bodyTop = nodeTop + sourceMeasurements.bodyTop;
      const bodyBottom = nodeTop + sourceMeasurements.bodyBottom;
      const isRowVisible =
        !sourceMeasurements.hiddenHandleIds.includes(sourceHandleId) &&
        sourceY >= bodyTop &&
        sourceY <= bodyBottom;

      if (!isRowVisible) {
        anchorX = positionAbsolute.x + sourceMeasurements.badgeAnchorX;
        anchorY = nodeTop + sourceMeasurements.badgeAnchorY;
      }
    }
  }

  const [path] = getBezierPath({
    sourceX: anchorX,
    sourceY: anchorY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return <BaseEdge id={id} path={path} style={style} markerEnd={markerEnd} />;
};

export default PipelineCanvasEdge;
