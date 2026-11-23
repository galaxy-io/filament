import type { FC, MouseEvent } from "react";

import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectorTileBadgeMark from "@/components/connections/ConnectorTileBadgeMark";
import ConnectorTileFrame from "@/components/connections/ConnectorTileFrame";
import {
  CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP,
  CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP,
} from "@/components/connections/constants";
import { useConnectorLogoURL } from "@/components/connections/hooks/useConnectorLogoURL";
import { type ConnectorTileBadge, ConnectorTileSize } from "@/components/connections/types";

const ConnectorLogo = styled.img<{ $height: number }>`
  display: block;
  width: ${({ $height }) => $height}px;
  height: ${({ $height }) => $height}px;
  object-fit: contain;
`;

const ConnectorLogoTileBadged = styled.span`
  position: relative;
  display: flex;
  flex-shrink: 0;
`;

const ConnectorLogoTileButton = styled.button`
  ${INTERACTIVE_RESET}
  display: flex;
  flex-shrink: 0;
  border-radius: ${t.radius.md};
  ${FOCUS_RING}
`;

interface ConnectorLogoTileProps {
  name: string;
  darkLogoUrl?: ConnectorSpec["darkLogoUrl"];
  lightLogoUrl?: ConnectorSpec["lightLogoUrl"];
  icon?: PhosphorIcon;
  size?: ConnectorTileSize;
  onClick?: (e: MouseEvent) => void;
  isDeleted?: boolean;
  isLoading?: boolean;
  badge?: ConnectorTileBadge;
}

const ConnectorLogoTile: FC<ConnectorLogoTileProps> = ({
  name,
  darkLogoUrl,
  lightLogoUrl,
  icon,
  size = ConnectorTileSize.MEDIUM,
  onClick,
  isDeleted = false,
  isLoading = false,
  badge,
}) => {
  const { logoURL, handleLogoError } = useConnectorLogoURL(darkLogoUrl, lightLogoUrl);

  const renderFallback = () => {
    if (isLoading) {
      return null;
    }
    if (icon) {
      return (
        <Icon
          component={icon}
          size={CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP[size]}
          variant={IconVariant.SECONDARY}
        />
      );
    }
    return (
      <Text
        size={CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP[size]}
        variant={TextVariant.SECONDARY}
        family={FontFamily.MONO}
      >
        {name.charAt(0).toUpperCase()}
      </Text>
    );
  };

  const frame = (
    <ConnectorTileFrame
      $size={size}
      $isClickable={!!onClick}
      $isDeleted={isDeleted}
      $hasBadge={!!badge}
    >
      {logoURL ? (
        <ConnectorLogo
          src={logoURL}
          alt={`${name} logo`}
          $height={CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP[size]}
          onError={handleLogoError}
        />
      ) : (
        renderFallback()
      )}
    </ConnectorTileFrame>
  );

  const tile = badge ? (
    <ConnectorLogoTileBadged>
      {frame}
      <ConnectorTileBadgeMark badge={badge} size={size} />
    </ConnectorLogoTileBadged>
  ) : (
    frame
  );

  if (!onClick) {
    return tile;
  }

  return (
    <ConnectorLogoTileButton type="button" aria-label={name} onClick={onClick}>
      {tile}
    </ConnectorLogoTileButton>
  );
};

export default ConnectorLogoTile;
