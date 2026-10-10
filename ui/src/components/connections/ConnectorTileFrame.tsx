import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { CONNECTOR_TILE_SIZE_TO_SIZE_MAP } from "@/components/connections/constants";
import type { ConnectorTileSize } from "@/components/connections/types";

const ConnectorTileFrame = styled.div<{
  $size: ConnectorTileSize;
  $isClickable: boolean;
  $isDeleted: boolean;
}>`
  width: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_SIZE_MAP[$size]}px;
  height: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_SIZE_MAP[$size]}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  background-color: ${t.color.background.base};

  border-radius: ${t.radius.md};
  border: ${HAIRLINE_WIDTH} solid
    ${({ $isDeleted }) => ($isDeleted ? t.color.border.error : t.color.border.tertiary)};

  overflow: hidden;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "inherit")};

  transition: opacity ${t.duration.fast};

  &:hover {
    opacity: ${({ $isClickable }) => ($isClickable ? 0.8 : 1)};
  }
`;

export default ConnectorTileFrame;
