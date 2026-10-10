import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const EMPTY_GRAPHIC_GHOST_TILE_SIZE = 24;
const EMPTY_GRAPHIC_GHOST_BAR_HEIGHT = 12;

const EMPTY_GRAPHIC_WIDTH = 560;
const EMPTY_GRAPHIC_HEIGHT = 200;

const EmptyGraphic = styled.div`
  width: ${EMPTY_GRAPHIC_WIDTH}px;
  height: ${EMPTY_GRAPHIC_HEIGHT}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

export const EmptyGraphicGhostTile = styled.div`
  display: flex;
  opacity: 0.3;
`;

export const EmptyGraphicGhostTileFallback = styled.div`
  width: ${EMPTY_GRAPHIC_GHOST_TILE_SIZE}px;
  height: ${EMPTY_GRAPHIC_GHOST_TILE_SIZE}px;

  background-color: ${t.color.background.secondary};

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.md};
`;

export const EmptyGraphicGhostBar = styled.div<{ $width: number }>`
  width: ${({ $width }) => $width}px;
  height: ${EMPTY_GRAPHIC_GHOST_BAR_HEIGHT}px;

  background-color: ${t.color.background.secondary};

  border-radius: ${t.radius.sm};
`;

export default EmptyGraphic;
