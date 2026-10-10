import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

const EMPTY_GRAPHIC_GHOST_BAR_HEIGHT = 12;

const EmptyGraphicGhostBar = styled.div<{ $width: number }>`
  width: ${({ $width }) => $width}px;
  height: ${EMPTY_GRAPHIC_GHOST_BAR_HEIGHT}px;

  background-color: ${t.color.background.secondary};

  border-radius: ${t.radius.sm};
`;

export default EmptyGraphicGhostBar;
