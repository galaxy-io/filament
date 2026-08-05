import { useState } from "react";

import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { GalaxyTheme } from "@galaxy-io/dls/theme";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";

export enum ConnectorTileSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

const CONNECTOR_TILE_SIZE_TO_SIZE_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 20,
  [ConnectorTileSize.MEDIUM]: 24,
  [ConnectorTileSize.LARGE]: 36,
};

const CONNECTOR_TILE_SIZE_TO_RADIUS_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 4,
  [ConnectorTileSize.MEDIUM]: 5,
  [ConnectorTileSize.LARGE]: 6,
};

const CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 16,
  [ConnectorTileSize.MEDIUM]: 18,
  [ConnectorTileSize.LARGE]: 21,
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
  border: 0.5px solid ${({ theme }) => theme.color.border.tertiary};

  overflow: hidden;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "inherit")};

  transition: opacity 100ms ease;

  &:hover {
    opacity: ${({ $isClickable }) => ($isClickable ? 0.8 : 1)};
  }
`);

const ConnectorLogo = styled.img<{ $height: number }>`
  display: block;
  width: ${({ $height }) => $height}px;
  height: ${({ $height }) => $height}px;
  object-fit: contain;
`;

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
  const { activeTheme } = useGalaxyTheme();
  const resolvedSpec = useConnectorSpec(connector);
  const catalogSpec = spec ?? resolvedSpec;
  const logoURL = match(activeTheme)
    .with(GalaxyTheme.DARK, () => catalogSpec?.darkLogoUrl)
    .with(GalaxyTheme.LIGHT, () => catalogSpec?.lightLogoUrl)
    .exhaustive();
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
