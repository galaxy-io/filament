import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";
import {
  getPipelineCanvasNodeBorderColor,
  getPipelineCanvasNodeOutline,
} from "@/pages/pipelines/canvas/nodes/utils";

const PipelineCanvasNodeIsland = styled.div<{ $isSelected?: boolean; $isRunning?: boolean }>`
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px;

  background-color: ${t.color.background.primary};
  border: ${HAIRLINE_WIDTH} solid
    ${({ $isSelected = false, $isRunning = false }) =>
      getPipelineCanvasNodeBorderColor($isSelected, $isRunning)};
  border-radius: ${t.radius.lg};
  outline: ${({ $isSelected = false, $isRunning = false }) =>
    getPipelineCanvasNodeOutline($isSelected, $isRunning)};
  outline-offset: -${HAIRLINE_WIDTH};

  transition: border-color ${t.duration.fast};
`;

export default PipelineCanvasNodeIsland;
