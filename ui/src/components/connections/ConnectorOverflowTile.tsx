import type { FC } from "react";

import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import ConnectorTileFrame from "@/components/connections/ConnectorTileFrame";
import { ConnectorTileSize } from "@/components/connections/types";

interface ConnectorOverflowTileProps {
  count: number;
  size?: ConnectorTileSize;
}

const ConnectorOverflowTile: FC<ConnectorOverflowTileProps> = ({
  count,
  size = ConnectorTileSize.MEDIUM,
}) => (
  <ConnectorTileFrame $size={size} $isClickable={false} $isDeleted={false}>
    <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} family={FontFamily.MONO}>
      +{count}
    </Text>
  </ConnectorTileFrame>
);

export default ConnectorOverflowTile;
