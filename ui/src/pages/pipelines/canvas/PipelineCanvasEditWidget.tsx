import { styled } from "@linaria/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import EditWidgetSelectorBody from "@/pages/pipelines/canvas/edit/EditWidgetSelectorBody";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/hooks";
import PipelineCanvasEditWidgetButton from "@/pages/pipelines/canvas/PipelineCanvasEditWidgetButton";
import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/types";

const PipelineCanvasEditWidgetContainer = withTheme(styled.div<PropsWithTheme>`
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

const PipelineCanvasEditWidget = () => {
  const { state, dispatch } = usePipelineCanvas();

  const handleModeToggle = (mode: PipelineCanvasEditMode) => {
    dispatch({
      type: PipelineCanvasActionType.SET_ACTIVE_MODE,
      payload: state.activeMode === mode ? null : mode,
    });
  };

  const handleDropdownClose = () => {
    dispatch({
      type: PipelineCanvasActionType.SET_ACTIVE_MODE,
      payload: null,
    });
  };

  return (
    <PipelineCanvasEditWidgetContainer>
      <FlexWrapper direction={FlexDirection.COLUMN} alignItems={AlignItems.CENTER} gap={12}>
        <Dropdown
          position={DropdownPosition.RIGHT_START}
          isOpen={state.activeMode === PipelineCanvasEditMode.ADD_NODE}
          onClose={handleDropdownClose}
          body={<EditWidgetSelectorBody />}
          offset={[-8, 16]}
          noPadding
        >
          <PipelineCanvasEditWidgetButton
            mode={PipelineCanvasEditMode.ADD_NODE}
            isActive={state.activeMode === PipelineCanvasEditMode.ADD_NODE}
            onClick={() => handleModeToggle(PipelineCanvasEditMode.ADD_NODE)}
          />
        </Dropdown>
        <PipelineCanvasEditWidgetButton
          mode={PipelineCanvasEditMode.ADD_EDGE}
          isActive={state.activeMode === PipelineCanvasEditMode.ADD_EDGE}
          onClick={() => handleModeToggle(PipelineCanvasEditMode.ADD_EDGE)}
        />
        <PipelineCanvasEditWidgetButton
          mode={PipelineCanvasEditMode.GRAB}
          isActive={state.activeMode === PipelineCanvasEditMode.GRAB}
          onClick={() => handleModeToggle(PipelineCanvasEditMode.GRAB)}
        />
      </FlexWrapper>
    </PipelineCanvasEditWidgetContainer>
  );
};

export default PipelineCanvasEditWidget;
