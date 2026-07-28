import { BaseEdge, type EdgeProps, getBezierPath, useInternalNode, useStore } from "@xyflow/react";

import { PIPELINE_NODE_SOURCE_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { PIPELINE_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

const zoomSelector = (state: { transform: [number, number, number] }) => state.transform[2];

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
  const zoom = useStore(zoomSelector);

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
    if (isTableEdge && zoom > 0) {
      const nodeElement = document.querySelector(`.react-flow__node[data-id="${source}"]`);
      const listElement = nodeElement?.querySelector("[data-table-list]");
      const badgeElement = nodeElement?.querySelector("[data-resource-badge]");

      if (nodeElement && listElement && badgeElement) {
        const nodeRect = nodeElement.getBoundingClientRect();
        const listRect = listElement.getBoundingClientRect();
        const badgeRect = badgeElement.getBoundingClientRect();

        const toFlowY = (clientY: number) => nodeTop + (clientY - nodeRect.top) / zoom;
        const listTop = toFlowY(listRect.top);
        const listBottom = toFlowY(listRect.bottom);
        const isRowVisible = sourceY >= listTop && sourceY <= listBottom;

        if (!isRowVisible) {
          anchorX = nodeRight;
          anchorY = toFlowY(badgeRect.top + badgeRect.height / 2);
        }
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
