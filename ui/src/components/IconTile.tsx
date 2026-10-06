import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const TileWrapper = styled.div<{ $size: number }>`
  width: ${({ $size }) => $size}px;
  height: ${({ $size }) => $size}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  border: 0.5px solid ${t.color.border.primary};
  border-radius: 6px;
`;

interface IconTileProps {
  icon: PhosphorIcon;
  size?: number;
  variant?: IconVariant;
}

const IconTile = ({ icon, size = 32, variant = IconVariant.SECONDARY }: IconTileProps) => (
  <TileWrapper $size={size}>
    <Icon component={icon} size={size / 2} variant={variant} />
  </TileWrapper>
);

export default IconTile;
