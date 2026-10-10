import { type FC, type MouseEvent, useState } from "react";

import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily, GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectorTileFrame from "@/components/connections/ConnectorTileFrame";
import {
  CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP,
  CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP,
} from "@/components/connections/constants";
import { ConnectorTileSize } from "@/components/connections/types";

const ConnectorLogo = styled.img<{ $height: number }>`
  display: block;
  width: ${({ $height }) => $height}px;
  height: ${({ $height }) => $height}px;
  object-fit: contain;
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
}

interface ConnectorLogoTileState {
  failedLogoURL?: string;
}

const DEFAULT_STATE: ConnectorLogoTileState = {};

const ConnectorLogoTile: FC<ConnectorLogoTileProps> = ({
  name,
  darkLogoUrl,
  lightLogoUrl,
  icon,
  size = ConnectorTileSize.MEDIUM,
  onClick,
  isDeleted = false,
  isLoading = false,
}) => {
  const { activeTheme } = useGalaxyTheme();
  const logoURL = match(activeTheme)
    .with(GalaxyTheme.DARK, () => darkLogoUrl)
    .with(GalaxyTheme.LIGHT, () => lightLogoUrl)
    .exhaustive();
  const [state, setState] = useState<ConnectorLogoTileState>(DEFAULT_STATE);
  const showLogo = !!logoURL && state.failedLogoURL !== logoURL;

  const handleLogoError = () => {
    setState((prev) => ({ ...prev, failedLogoURL: logoURL }));
  };

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

  return (
    <ConnectorTileFrame
      $size={size}
      $isClickable={!!onClick}
      onClick={onClick}
      $isDeleted={isDeleted}
    >
      {showLogo ? (
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
};

export default ConnectorLogoTile;
