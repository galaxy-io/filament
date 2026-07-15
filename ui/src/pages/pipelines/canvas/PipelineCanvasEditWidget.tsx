import { styled } from "@linaria/react";
import { PencilSimpleIcon, PlusIcon, PulseIcon } from "@phosphor-icons/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/types";

const WidgetContainer = withTheme(styled.div<PropsWithTheme>`
  position: absolute;
  left: 16px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 1001;

  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px;

  background-color: ${({ theme }) => theme.color.background.base};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 200px;
`);

const ButtonGroup = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
`;

const Divider = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 0.5px;
  background-color: ${({ theme }) => theme.color.border.primary};
`);

const IconButton = withTheme(styled.button<PropsWithTheme<{ $isActive?: boolean }>>`
  width: 32px;
  height: 32px;
  padding: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme, $isActive }) =>
    $isActive ? theme.color.background.primaryAlt : "transparent"};
  border: none;
  border-radius: 50%;
  cursor: pointer;

  transition: all 100ms ease;

  &:hover {
    background-color: ${({ theme, $isActive }) =>
      $isActive ? theme.color.background.primaryAlt : theme.color.background.tertiary};
  }
`);

interface PipelineCanvasEditWidgetProps {
  activeMode?: PipelineCanvasEditMode;
  onModeChange?: (mode: PipelineCanvasEditMode) => void;
}

const PipelineCanvasEditWidget = ({
  activeMode = PipelineCanvasEditMode.ADD,
  onModeChange,
}: PipelineCanvasEditWidgetProps) => {
  return (
    <WidgetContainer>
      <ButtonGroup>
        <IconButton
          $isActive={activeMode === PipelineCanvasEditMode.ADD}
          onClick={() => onModeChange?.(PipelineCanvasEditMode.ADD)}
        >
          <Icon
            component={PlusIcon}
            weight={
              activeMode === PipelineCanvasEditMode.ADD ? IconWeight.BOLD : IconWeight.REGULAR
            }
            variant={
              activeMode === PipelineCanvasEditMode.ADD
                ? IconVariant.PRIMARY_ALT
                : IconVariant.TERTIARY
            }
          />
        </IconButton>
        <IconButton
          $isActive={activeMode === PipelineCanvasEditMode.EDIT}
          onClick={() => onModeChange?.(PipelineCanvasEditMode.EDIT)}
        >
          <Icon
            component={PencilSimpleIcon}
            weight={
              activeMode === PipelineCanvasEditMode.EDIT ? IconWeight.BOLD : IconWeight.REGULAR
            }
            variant={activeMode === PipelineCanvasEditMode.EDIT ? undefined : IconVariant.TERTIARY}
          />
        </IconButton>
      </ButtonGroup>
      <Divider />
      <ButtonGroup>
        <IconButton
          $isActive={activeMode === PipelineCanvasEditMode.ACTIVITY}
          onClick={() => onModeChange?.(PipelineCanvasEditMode.ACTIVITY)}
        >
          <Icon
            component={PulseIcon}
            weight={
              activeMode === PipelineCanvasEditMode.ACTIVITY ? IconWeight.BOLD : IconWeight.REGULAR
            }
            variant={
              activeMode === PipelineCanvasEditMode.ACTIVITY ? undefined : IconVariant.TERTIARY
            }
          />
        </IconButton>
      </ButtonGroup>
    </WidgetContainer>
  );
};

export default PipelineCanvasEditWidget;
