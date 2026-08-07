import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

export const EMPTY_GRAPHIC_WIDTH = 560;
export const EMPTY_GRAPHIC_HEIGHT = 200;

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

export const EmptyGraphicGhostTileFallback = withTheme(styled.div<PropsWithTheme>`
  width: 24px;
  height: 24px;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 4px;
`);

export const EmptyGraphicGhostBar = withTheme(styled.div<PropsWithTheme<{ $width: number }>>`
  width: ${({ $width }) => $width}px;
  height: 12px;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border-radius: 4px;
`);

export default EmptyGraphic;
