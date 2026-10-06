import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";

const PipelineCanvasNodeIsland = styled.div<{ $isSelected?: boolean }>`
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px;

  background-color: ${t.color.background.primary};
  border: ${HAIRLINE_WIDTH} solid
    ${({ $isSelected }) =>
      $isSelected ? t.color.solid.primary.background : t.color.border.primary};
  border-radius: ${t.radius.lg};
  outline: ${({ $isSelected }) =>
    $isSelected ? `1px solid ${t.color.solid.primary.background}` : "none"};
  outline-offset: -1px;

  transition: border-color 100ms ease;
`;

export default PipelineCanvasNodeIsland;
