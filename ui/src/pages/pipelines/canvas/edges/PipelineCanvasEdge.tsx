import { useSyncExternalStore } from "react";

import { BaseEdge, type EdgeProps, getBezierPath, useInternalNode } from "@xyflow/react";

import { PIPELINE_NODE_SOURCE_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { PIPELINE_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";
import {
  getPipelineNodeMeasurements,
  subscribePipelineNodeMeasurements,
} from "@/pages/pipelines/canvas/nodes/measurements";

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
  const sourceMeasurements = useSyncExternalStore(subscribePipelineNodeMeasurements, () =>
    getPipelineNodeMeasurements(source),
  );

  let anchorX = sourceX;
  let anchorY = sourceY;

  const { positionAbsolute } = sourceNode?.internals ?? {};
  const { width, height } = sourceNode?.measured ?? {};
  if (positionAbsolute && width && height) {
    const nodeTop = positionAbsolute.y;
    const nodeBottom = nodeTop + height;
    const nodeRight = positionAbsolute.x + width;

    anchorX = Math.min(sourceX, nodeRight);
    anchorY = clamp(sourceY, nodeTop + PIPELINE_NODE_PADDING, nodeBottom - PIPELINE_NODE_PADDING);

    const isTableEdge = sourceHandleId && sourceHandleId !== PIPELINE_NODE_SOURCE_HANDLE_ID;
    if (isTableEdge && sourceMeasurements && sourceMeasurements.badgeCenterY !== null) {
      const listTop = nodeTop + sourceMeasurements.listTop;
      const listBottom = nodeTop + sourceMeasurements.listBottom;
      const isRowVisible = sourceY >= listTop && sourceY <= listBottom;

      if (!isRowVisible) {
        anchorX = nodeRight;
        anchorY = nodeTop + sourceMeasurements.badgeCenterY;
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
