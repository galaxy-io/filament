import {
  BaseEdge,
  EdgeLabelRenderer,
  type EdgeProps,
  getBezierPath,
  useInternalNode,
} from "@xyflow/react";

import { IngestionType } from "@/gen/ingestion/v1/common_pb";

import PipelineCanvasEdgeModeLabel from "@/pages/pipelines/canvas/edges/PipelineCanvasEdgeModeLabel";
import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

const PipelineCanvasEdge = ({
  id,
  source,
  target,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style,
  markerEnd,
  data,
  selected,
}: EdgeProps<CanvasEdge>) => {
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

  const [path, labelX, labelY] = getBezierPath({
    sourceX: anchorX,
    sourceY: anchorY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return (
    <>
      <BaseEdge id={id} path={path} style={style} markerEnd={markerEnd} />
      <EdgeLabelRenderer>
        <PipelineCanvasEdgeModeLabel
          edgeId={id}
          sourceNodeId={source}
          targetNodeId={target}
          ingestionType={data?.ingestionType ?? IngestionType.SNAPSHOT_REPLACE}
          isSelected={Boolean(selected)}
          labelX={labelX}
          labelY={labelY}
        />
      </EdgeLabelRenderer>
    </>
  );
};

export default PipelineCanvasEdge;
