import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_NODE_BORDER_RADIUS,
  PIPELINE_NODE_PADDING,
} from "@/pages/pipelines/canvas/nodes/constants";

const PipelineNodeIsland = withTheme(styled.div<PropsWithTheme<{ $isSelected?: boolean }>>`
  padding: ${PIPELINE_NODE_PADDING}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid
    ${({ theme, $isSelected }) =>
      $isSelected ? theme.color.background.galaxy : theme.color.border.primary};
  border-radius: ${PIPELINE_NODE_BORDER_RADIUS}px;
  outline: ${({ theme, $isSelected }) =>
    $isSelected ? `1px solid ${theme.color.background.galaxy}` : "none"};
  outline-offset: -1px;

  transition: border-color 100ms ease;
`);

export default PipelineNodeIsland;
