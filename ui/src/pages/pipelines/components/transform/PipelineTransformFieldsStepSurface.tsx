import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const PipelineTransformFieldsStepSurface = withTheme(styled.div<
  PropsWithTheme<{ $isHoverable: boolean }>
>`
  width: 100%;
  min-width: 0;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme, $isHoverable }) =>
      $isHoverable ? theme.color.background.secondary : "transparent"};
  }
`);

export default PipelineTransformFieldsStepSurface;
