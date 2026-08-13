import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const StyledButton = withTheme(
  styled.button<PropsWithTheme<{ $isActive: boolean }>>`
    width: 32px;
    height: 32px;
    padding: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: ${({ theme, $isActive }) =>
      $isActive ? theme.color.background.primaryAlt : theme.color.background.galaxy};
    border: none;
    border-radius: 50%;
    cursor: pointer;

    transition: background-color 100ms ease;

    &:hover {
      background-color: ${({ theme, $isActive }) =>
        $isActive ? theme.color.background.primaryAlt : theme.color.background.galaxyAlt};
    }
  `,
);

interface PipelineCanvasEditWidgetButtonProps {
  icon: PhosphorIcon;
  isActive: boolean;
  onClick: () => void;
}

const PipelineCanvasEditWidgetButton = ({
  icon,
  isActive,
  onClick,
}: PipelineCanvasEditWidgetButtonProps) => (
  <StyledButton $isActive={isActive} onClick={onClick}>
    <Icon component={icon} variant={isActive ? IconVariant.PRIMARY_ALT : IconVariant.PRIMARY} />
  </StyledButton>
);

export default PipelineCanvasEditWidgetButton;
