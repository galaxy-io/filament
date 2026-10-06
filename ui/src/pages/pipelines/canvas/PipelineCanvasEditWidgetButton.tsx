import { type ButtonHTMLAttributes, forwardRef } from "react";

import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const StyledButton = styled.button<{ $isActive: boolean }>`
    width: 32px;
    height: 32px;
    padding: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    color: ${t.color.solid.primary.text};
    background-color: ${({ $isActive }) =>
      $isActive ? t.color.solid.primary.pressed : t.color.solid.primary.background};
    border: none;
    border-radius: 50%;
    cursor: pointer;

    transition: background-color 100ms ease;

    &:hover {
      background-color: ${({ $isActive }) =>
        $isActive ? t.color.solid.primary.pressed : t.color.solid.primary.hovered};
    }
  `;

interface PipelineCanvasEditWidgetButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: PhosphorIcon;
  isActive: boolean;
}

const PipelineCanvasEditWidgetButton = forwardRef<
  HTMLButtonElement,
  PipelineCanvasEditWidgetButtonProps
>(({ icon, isActive, ...rest }, ref) => (
  <StyledButton ref={ref} type="button" $isActive={isActive} {...rest}>
    <Icon component={icon} variant={IconVariant.INHERIT} />
  </StyledButton>
));

export default PipelineCanvasEditWidgetButton;
