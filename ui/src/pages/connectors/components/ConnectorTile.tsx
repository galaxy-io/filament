import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Skeleton, { SkeletonSize, SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily, GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type ConnectorSpec, GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import { useGetConnectorQuery } from "@/api/queries/connectors";

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

const CONNECTOR_TILE_SIZE_TO_SKELETON_SIZE_MAP: Record<ConnectorTileSize, SkeletonSize> = {
  [ConnectorTileSize.SMALL]: SkeletonSize.X_SMALL,
  [ConnectorTileSize.MEDIUM]: SkeletonSize.SMALL,
  [ConnectorTileSize.LARGE]: SkeletonSize.LARGE,
};

const CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 14,
  [ConnectorTileSize.MEDIUM]: 16,
  [ConnectorTileSize.LARGE]: 24,
};

const CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP: Record<ConnectorTileSize, TextSize> = {
  [ConnectorTileSize.SMALL]: TextSize.CAPTION,
  [ConnectorTileSize.MEDIUM]: TextSize.BODY_MD,
  [ConnectorTileSize.LARGE]: TextSize.BODY_LG,
};

const TileWrapper = styled.div<{
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

  background-color: transparent;

  border-radius: ${t.radius.md};
  border: 0.5px solid
    ${({ $isDeleted }) => ($isDeleted ? t.color.border.error : t.color.border.tertiary)};

  overflow: hidden;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "inherit")};

  transition: opacity 100ms ease;

  &:hover {
    opacity: ${({ $isClickable }) => ($isClickable ? 0.8 : 1)};
  }
`;

const ConnectorLogo = styled.img<{ $height: number }>`
  display: block;
  width: ${({ $height }) => $height}px;
  height: ${({ $height }) => $height}px;
  object-fit: contain;
`;

interface ConnectorTileProps {
  connector: ConnectorSpec["name"];
  kind?: ConnectorKind;
  size?: ConnectorTileSize;
  onClick?: (e: React.MouseEvent) => void;
  isDeleted?: boolean;
}

interface ConnectorTileState {
  failedLogoURL?: string;
}

const DEFAULT_STATE: ConnectorTileState = {};

const ConnectorTile = ({
  connector,
  kind,
  size = ConnectorTileSize.MEDIUM,
  onClick,
  isDeleted = false,
}: ConnectorTileProps) => {
  const { activeTheme } = useGalaxyTheme();
  const { data, isLoading } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, { connector, kind }),
    options: { enabled: !!connector && !!kind },
  });
  const catalogSpec = data?.connector;
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
    <TileWrapper $size={size} $isClickable={!!onClick} onClick={onClick} $isDeleted={isDeleted}>
      {showLogo ? (
        <ConnectorLogo
          src={logoURL}
          alt={`${catalogSpec?.displayName || connector} logo`}
          $height={CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP[size]}
          onError={handleLogoError}
        />
      ) : isLoading ? null : (
        <Text
          size={CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP[size]}
          variant={TextVariant.SECONDARY}
          family={FontFamily.MONO}
        >
          {connector.charAt(0).toUpperCase()}
        </Text>
      )}
    </TileWrapper>
  );
};

export const ConnectorOverflowTile = ({
  count,
  size = ConnectorTileSize.MEDIUM,
}: {
  count: number;
  size?: ConnectorTileSize;
}) => {
  return (
    <TileWrapper $size={size} $isClickable={false} $isDeleted={false}>
      <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} family={FontFamily.MONO}>
        +{count}
      </Text>
    </TileWrapper>
  );
};

export const ConnectorTileShimmer = ({
  size = ConnectorTileSize.MEDIUM,
}: {
  size?: ConnectorTileSize;
}) => {
  return (
    <Box width={CONNECTOR_TILE_SIZE_TO_SIZE_MAP[size]}>
      <Skeleton
        variant={SkeletonVariant.RECT}
        size={CONNECTOR_TILE_SIZE_TO_SKELETON_SIZE_MAP[size]}
      />
    </Box>
  );
};

export default ConnectorTile;
