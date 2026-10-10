import type { FC } from "react";

import Skeleton, { SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";

import {
  CONNECTOR_TILE_SIZE_TO_SIZE_MAP,
  CONNECTOR_TILE_SIZE_TO_SKELETON_SIZE_MAP,
} from "@/components/connections/constants";
import { ConnectorTileSize } from "@/components/connections/types";

interface ConnectorTileShimmerProps {
  size?: ConnectorTileSize;
}

const ConnectorTileShimmer: FC<ConnectorTileShimmerProps> = ({
  size = ConnectorTileSize.MEDIUM,
}) => (
  <Box width={CONNECTOR_TILE_SIZE_TO_SIZE_MAP[size]}>
    <Skeleton
      variant={SkeletonVariant.RECT}
      size={CONNECTOR_TILE_SIZE_TO_SKELETON_SIZE_MAP[size]}
    />
  </Box>
);

export default ConnectorTileShimmer;
