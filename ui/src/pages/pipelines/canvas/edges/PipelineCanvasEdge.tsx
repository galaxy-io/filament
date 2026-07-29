import { BaseEdge, type EdgeProps, getBezierPath, useInternalNode } from "@xyflow/react";

import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

const PipelineCanvasEdge = ({
  id,
  source,
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

  let anchorX = sourceX;
  let anchorY = sourceY;

  const { positionAbsolute } = sourceNode?.internals ?? {};
  const { width, height } = sourceNode?.measured ?? {};
  if (positionAbsolute && width && height) {
    const nodeTop = positionAbsolute.y;
    const nodeBottom = nodeTop + height;
    const nodeRight = positionAbsolute.x + width;

    anchorX = Math.min(sourceX, nodeRight);
    anchorY = clamp(
      sourceY,
      nodeTop + PIPELINE_CANVAS_NODE_PADDING,
      nodeBottom - PIPELINE_CANVAS_NODE_PADDING,
    );
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
