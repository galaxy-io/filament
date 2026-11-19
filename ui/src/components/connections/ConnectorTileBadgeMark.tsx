import type { FC } from "react";

import { styled } from "@linaria/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import {
  CONNECTOR_TILE_BADGE_TO_ICON_MAP,
  CONNECTOR_TILE_BADGE_TO_LABEL_MAP,
  CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP,
} from "@/components/connections/constants";
import type { ConnectorTileBadge, ConnectorTileSize } from "@/components/connections/types";

const ConnectorTileBadgeCircle = styled.span<{ $size: ConnectorTileSize }>`
  position: absolute;
  right: ${({ $size }) => -CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP[$size].offset}px;
  bottom: ${({ $size }) => -CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP[$size].offset}px;
  width: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP[$size].size}px;
  height: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP[$size].size}px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  border-radius: ${t.radius.pill};
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.tertiary};
  background-color: ${t.color.background.secondary};
`;

interface ConnectorTileBadgeMarkProps {
  badge: ConnectorTileBadge;
  size: ConnectorTileSize;
}

const ConnectorTileBadgeMark: FC<ConnectorTileBadgeMarkProps> = ({ badge, size }) => (
  <Tooltip body={CONNECTOR_TILE_BADGE_TO_LABEL_MAP[badge]}>
    <ConnectorTileBadgeCircle
      $size={size}
      role="img"
      aria-label={CONNECTOR_TILE_BADGE_TO_LABEL_MAP[badge]}
    >
      <Icon
        component={CONNECTOR_TILE_BADGE_TO_ICON_MAP[badge]}
        size={CONNECTOR_TILE_SIZE_TO_BADGE_GEOMETRY_MAP[size].iconSize}
        variant={IconVariant.SECONDARY}
      />
    </ConnectorTileBadgeCircle>
  </Tooltip>
);

export default ConnectorTileBadgeMark;
