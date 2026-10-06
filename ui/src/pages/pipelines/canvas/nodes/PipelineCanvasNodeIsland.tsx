import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_CANVAS_NODE_BORDER_RADIUS,
  PIPELINE_CANVAS_NODE_PADDING,
} from "@/pages/pipelines/canvas/nodes/constants";

const PipelineCanvasNodeIsland = styled.div<{ $isSelected?: boolean }>`
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px;

  background-color: ${t.color.background.primary};
  border: 0.5px solid
    ${({ $isSelected }) =>
      $isSelected ? t.color.solid.primary.background : t.color.border.primary};
  border-radius: ${PIPELINE_CANVAS_NODE_BORDER_RADIUS}px;
  outline: ${({ $isSelected }) =>
    $isSelected ? `1px solid ${t.color.solid.primary.background}` : "none"};
  outline-offset: -1px;

  transition: border-color 100ms ease;
`;

export default PipelineCanvasNodeIsland;
