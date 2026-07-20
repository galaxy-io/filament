import { styled } from "@linaria/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP } from "@/pages/pipelines/canvas/constants";
import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/types";

const getIconProps = (mode: PipelineCanvasEditMode, isActive: boolean) => {
  if (mode === PipelineCanvasEditMode.ADD_NODE) {
    return { variant: isActive ? IconVariant.PRIMARY_ALT : IconVariant.PRIMARY };
  }
  return { variant: isActive ? IconVariant.PRIMARY : IconVariant.TERTIARY };
};

const StyledButton = withTheme(
  styled.button<PropsWithTheme<{ $mode: PipelineCanvasEditMode; $isActive: boolean }>>`
    width: 32px;
    height: 32px;
    padding: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: ${({ theme, $mode, $isActive }) => {
      if ($mode === PipelineCanvasEditMode.ADD_NODE) {
        return $isActive ? theme.color.background.primaryAlt : theme.color.background.galaxy;
      }
      return $isActive ? theme.color.background.secondary : theme.color.background.base;
    }};
    border: none;
    border-radius: 50%;
    cursor: pointer;

    transition: background-color 100ms ease;

    &:hover {
      background-color: ${({ theme, $mode, $isActive }) => {
        if ($mode === PipelineCanvasEditMode.ADD_NODE) {
          return $isActive ? theme.color.background.primaryAlt : theme.color.background.galaxyAlt;
        }
        return theme.color.background.secondary;
      }};
    }
  `,
);

interface PipelineCanvasEditWidgetButtonProps {
  mode: PipelineCanvasEditMode;
  isActive: boolean;
  onClick: () => void;
}

const PipelineCanvasEditWidgetButton = ({
  mode,
  isActive,
  onClick,
}: PipelineCanvasEditWidgetButtonProps) => (
  <StyledButton $mode={mode} $isActive={isActive} onClick={onClick}>
    <Icon
      component={PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP[mode]}
      {...getIconProps(mode, isActive)}
    />
  </StyledButton>
);

export default PipelineCanvasEditWidgetButton;
