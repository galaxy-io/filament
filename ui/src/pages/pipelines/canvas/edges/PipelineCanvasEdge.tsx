import { useSyncExternalStore } from "react";

import { BaseEdge, type EdgeProps, getBezierPath, useInternalNode } from "@xyflow/react";

import { getPipelineCanvasEdgeAnchor } from "@/pages/pipelines/canvas/edges/utils";
import {
  getPipelineCanvasNodeMeasurements,
  subscribePipelineCanvasNodeMeasurements,
} from "@/pages/pipelines/canvas/nodes/utils";

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
  const measurements = useSyncExternalStore(subscribePipelineCanvasNodeMeasurements, () =>
    getPipelineCanvasNodeMeasurements(source),
  );

  const anchor = getPipelineCanvasEdgeAnchor({
    sourceNode,
    sourceHandle: sourceHandleId,
    sourceX,
    sourceY,
    measurements,
  });

  const [path] = getBezierPath({
    sourceX: anchor.x,
    sourceY: anchor.y,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return <BaseEdge id={id} path={path} style={style} markerEnd={markerEnd} />;
};

export default PipelineCanvasEdge;
