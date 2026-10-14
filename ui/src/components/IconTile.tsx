import type { FC } from "react";

import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const TileWrapper = styled.div<{ $size: number }>`
  width: ${({ $size }) => $size}px;
  height: ${({ $size }) => $size}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.md};
`;

interface IconTileProps {
  icon: PhosphorIcon;
  size?: number;
  variant?: IconVariant;
}

const IconTile: FC<IconTileProps> = ({ icon, size = 32, variant = IconVariant.SECONDARY }) => (
  <TileWrapper $size={size}>
    <Icon component={icon} size={size / 2} variant={variant} />
  </TileWrapper>
);

export default IconTile;
