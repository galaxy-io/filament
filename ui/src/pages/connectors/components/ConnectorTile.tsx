import { useState } from "react";

import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import CellGridBackground from "@galaxy-io/dls/backgrounds/CellGridBackground";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { GalaxyTheme } from "@galaxy-io/dls/theme/constants";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { useConnectorSpec } from "@/pages/connectors/hooks";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export enum ConnectorTileSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

const getTileSize = (size: ConnectorTileSize): number =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => 24)
    .with(ConnectorTileSize.MEDIUM, () => 32)
    .with(ConnectorTileSize.LARGE, () => 40)
    .exhaustive();

const getTileRadius = (size: ConnectorTileSize): number =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => 4)
    .with(ConnectorTileSize.MEDIUM, () => 5)
    .with(ConnectorTileSize.LARGE, () => 6)
    .exhaustive();

const getLogoHeight = (size: ConnectorTileSize): number =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => 16)
    .with(ConnectorTileSize.MEDIUM, () => 20)
    .with(ConnectorTileSize.LARGE, () => 24)
    .exhaustive();

const getTextSize = (size: ConnectorTileSize): TextSize =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => TextSize.CAPTION)
    .with(ConnectorTileSize.MEDIUM, () => TextSize.BODY_MD)
    .with(ConnectorTileSize.LARGE, () => TextSize.BODY_LG)
    .exhaustive();

const TileWrapper = withTheme(styled.div<
  PropsWithTheme<{ $size: ConnectorTileSize; $isClickable: boolean }>
>`
  width: ${({ $size }) => getTileSize($size)}px;
  height: ${({ $size }) => getTileSize($size)}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border-radius: ${({ $size }) => getTileRadius($size)}px;

  overflow: hidden;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "inherit")};

  transition: opacity 100ms ease;

  &:hover {
    opacity: ${({ $isClickable }) => ($isClickable ? 0.8 : 1)};
  }
`);

const EmptyTileWrapper = withTheme(styled.div<PropsWithTheme<{ $size: ConnectorTileSize }>>`
  width: ${({ $size }) => getTileSize($size)}px;
  height: ${({ $size }) => getTileSize($size)}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: transparent;

  border: 0.5px dashed ${({ theme }) => theme.color.border.tertiary};
  border-radius: ${({ $size }) => getTileRadius($size)}px;

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
      <CellGridBackground cellSize={3} strokeWidth={1} lineOpacity={0.5} fillContainer />
    </EmptyTileWrapper>
  );
};

interface ConnectorTileProps {
  connector: string;
  spec?: ConnectorSpec;
  size?: ConnectorTileSize;
  onClick?: (e: React.MouseEvent) => void;
}

const ConnectorTile = ({
  connector,
  spec,
  size = ConnectorTileSize.MEDIUM,
  onClick,
}: ConnectorTileProps) => {
  const { activeTheme } = useGalaxyTheme();
  const resolvedSpec = useConnectorSpec(connector);
  const catalogSpec = spec ?? resolvedSpec;
  const logoURL =
    activeTheme === GalaxyTheme.DARK ? catalogSpec?.darkLogoUrl : catalogSpec?.lightLogoUrl;
  const [failedLogoURL, setFailedLogoURL] = useState<string>();
  const showLogo = !!logoURL && failedLogoURL !== logoURL;

  return (
    <TileWrapper $size={size} $isClickable={!!onClick} onClick={onClick}>
      {showLogo ? (
        <ConnectorLogo
          src={logoURL}
          alt={`${catalogSpec?.displayName || connector} logo`}
          $height={getLogoHeight(size)}
          onError={() => setFailedLogoURL(logoURL)}
        />
      ) : (
        <Text size={getTextSize(size)} variant={TextVariant.SECONDARY} isMonospace>
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
