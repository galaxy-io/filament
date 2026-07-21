import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const getIconVariant = (isActive: boolean, isPrimary: boolean) => {
  if (isPrimary) {
    return isActive ? IconVariant.PRIMARY_ALT : IconVariant.PRIMARY;
  }
  return isActive ? IconVariant.PRIMARY : IconVariant.TERTIARY;
};

const StyledButton = withTheme(
  styled.button<PropsWithTheme<{ $isActive: boolean; $isPrimary: boolean }>>`
    width: 32px;
    height: 32px;
    padding: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: ${({ theme, $isActive, $isPrimary }) => {
      if ($isPrimary) {
        return $isActive ? theme.color.background.primaryAlt : theme.color.background.galaxy;
      }
      return $isActive ? theme.color.background.secondary : theme.color.background.base;
    }};
    border: none;
    border-radius: 50%;
    cursor: pointer;

    transition: background-color 100ms ease;

    &:hover {
      background-color: ${({ theme, $isActive, $isPrimary }) => {
        if ($isPrimary) {
          return $isActive ? theme.color.background.primaryAlt : theme.color.background.galaxyAlt;
        }
        return theme.color.background.secondary;
      }};
    }
  `,
);

interface PipelineCanvasEditWidgetButtonProps {
  icon: PhosphorIcon;
  isActive: boolean;
  onClick: () => void;
  isPrimary?: boolean;
}

const PipelineCanvasEditWidgetButton = ({
  icon,
  isActive,
  onClick,
  isPrimary = false,
}: PipelineCanvasEditWidgetButtonProps) => (
  <StyledButton $isActive={isActive} $isPrimary={isPrimary} onClick={onClick}>
    <Icon component={icon} variant={getIconVariant(isActive, isPrimary)} />
  </StyledButton>
);

export default PipelineCanvasEditWidgetButton;
