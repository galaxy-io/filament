import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const EMPTY_GRAPHIC_GHOST_TILE_SIZE = 24;

const EmptyGraphicGhostTileFallback = styled.div`
  width: ${EMPTY_GRAPHIC_GHOST_TILE_SIZE}px;
  height: ${EMPTY_GRAPHIC_GHOST_TILE_SIZE}px;

  background-color: ${t.color.background.secondary};

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.md};
`;

export default EmptyGraphicGhostTileFallback;
