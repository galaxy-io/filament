import { useState } from "react";

import { styled } from "@linaria/react";
import { CircleIcon } from "@phosphor-icons/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";

export enum ConnectorTileSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

const CONNECTOR_TILE_SIZE_TO_SIZE_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 24,
  [ConnectorTileSize.MEDIUM]: 32,
  [ConnectorTileSize.LARGE]: 40,
};

const CONNECTOR_TILE_SIZE_TO_RADIUS_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 4,
  [ConnectorTileSize.MEDIUM]: 5,
  [ConnectorTileSize.LARGE]: 6,
};

const CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 16,
  [ConnectorTileSize.MEDIUM]: 20,
  [ConnectorTileSize.LARGE]: 24,
};

const CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP: Record<ConnectorTileSize, TextSize> = {
  [ConnectorTileSize.SMALL]: TextSize.CAPTION,
  [ConnectorTileSize.MEDIUM]: TextSize.BODY_MD,
  [ConnectorTileSize.LARGE]: TextSize.BODY_LG,
};

const TileWrapper = withTheme(styled.div<
  PropsWithTheme<{ $size: ConnectorTileSize; $isClickable: boolean }>
>`
  width: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_SIZE_MAP[$size]}px;
  height: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_SIZE_MAP[$size]}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border-radius: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_RADIUS_MAP[$size]}px;
  border: 0.5px solid ${({ theme }) => theme.color.border.secondary};

  overflow: hidden;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "inherit")};

  transition: opacity 100ms ease;

  &:hover {
    opacity: ${({ $isClickable }) => ($isClickable ? 0.8 : 1)};
  }
`);

const EmptyTileWrapper = withTheme(styled.div<PropsWithTheme<{ $size: ConnectorTileSize }>>`
  width: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_SIZE_MAP[$size]}px;
  height: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_SIZE_MAP[$size]}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.error};
  border: 0.5px solid ${({ theme }) => theme.color.border.error};
  border-radius: ${({ $size }) => CONNECTOR_TILE_SIZE_TO_RADIUS_MAP[$size]}px;

  overflow: hidden;
`);

const ConnectorLogo = styled.img<{ $height: number }>`
  display: block;
  width: ${({ $height }) => $height}px;
  height: ${({ $height }) => $height}px;
  object-fit: contain;
`;

interface EmptyConnectorTileProps {
  size?: ConnectorTileSize;
}

export const ConnectorTileEmpty = ({
  size = ConnectorTileSize.MEDIUM,
}: EmptyConnectorTileProps) => {
  return (
    <EmptyTileWrapper $size={size}>
      <Icon
        component={CircleIcon}
        size={CONNECTOR_TILE_SIZE_TO_SIZE_MAP[size] / 2}
        variant={IconVariant.ERROR}
        weight={IconWeight.REGULAR}
      />
    </EmptyTileWrapper>
  );
};

interface ConnectorTileProps {
  connector: string;
  spec?: ConnectorSpec;
  size?: ConnectorTileSize;
  onClick?: (e: React.MouseEvent) => void;
}

interface ConnectorTileState {
  failedLogoURL?: string;
}

const DEFAULT_STATE: ConnectorTileState = {};

const ConnectorTile = ({
  connector,
  spec,
  size = ConnectorTileSize.MEDIUM,
  onClick,
}: ConnectorTileProps) => {
  const resolvedSpec = useConnectorSpec(connector);
  const catalogSpec = spec ?? resolvedSpec;
  const logoURL = catalogSpec?.darkLogoUrl;
  const [state, setState] = useState<ConnectorTileState>(DEFAULT_STATE);
  const showLogo = !!logoURL && state.failedLogoURL !== logoURL;

  const handleLogoError = () => {
    setState((prev) => ({ ...prev, failedLogoURL: logoURL }));
  };

  return (
    <TileWrapper $size={size} $isClickable={!!onClick} onClick={onClick}>
      {showLogo ? (
        <ConnectorLogo
          src={logoURL}
          alt={`${catalogSpec?.displayName || connector} logo`}
          $height={CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP[size]}
          onError={handleLogoError}
        />
      ) : (
        <Text
          size={CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP[size]}
          variant={TextVariant.SECONDARY}
          isMonospace
        >
          {connector.charAt(0).toUpperCase()}
        </Text>
      )}
    </TileWrapper>
  );
};

export const ConnectorOverflowTile = ({ count }: { count: number }) => {
  return (
    <TileWrapper $size={ConnectorTileSize.SMALL} $isClickable={false}>
      <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace>
        +{count}
      </Text>
    </TileWrapper>
  );
};

export default ConnectorTile;
