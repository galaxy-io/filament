import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

// @dls-migrate tokens.background.alt: Inverse is a scope, not a token: render the opposite-theme surface as `<GalaxyProvider isScoped theme={…}>` around a `Box variant`, and read the normal roles inside it.
const StyledButton = styled.button<{ $isActive: boolean }>`
    width: 32px;
    height: 32px;
    padding: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: ${({ $isActive }) =>
      $isActive ? t.color.background.primaryAlt : t.color.solid.primary.background};
    border: none;
    border-radius: 50%;
    cursor: pointer;

    transition: background-color 100ms ease;

    &:hover {
      background-color: ${({ $isActive }) =>
        $isActive ? t.color.background.primaryAlt : t.color.solid.primary.hovered};
    }
  `;

interface PipelineCanvasEditWidgetButtonProps {
  icon: PhosphorIcon;
  isActive: boolean;
  onClick: () => void;
}

const PipelineCanvasEditWidgetButton = ({
  icon,
  isActive,
  onClick,
}: PipelineCanvasEditWidgetButtonProps) => {
  const { activeTheme } = useGalaxyTheme();

  // @dls-migrate icon.IconVariant.PRIMARY_ALT: Inverse is a scope, not a member: wrap the content in `<GalaxyProvider isScoped theme={…}>` (the opposite theme) and use `IconVariant.PRIMARY` inside it.
  const iconVariant = match(activeTheme)
    .with(GalaxyTheme.LIGHT, () => IconVariant.PRIMARY_ALT)
    .with(GalaxyTheme.DARK, () => (isActive ? IconVariant.PRIMARY_ALT : IconVariant.PRIMARY))
    .exhaustive();

  return (
    <StyledButton $isActive={isActive} onClick={onClick}>
      <Icon component={icon} variant={iconVariant} />
    </StyledButton>
  );
};

export default PipelineCanvasEditWidgetButton;
