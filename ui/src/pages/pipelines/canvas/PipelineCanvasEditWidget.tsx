import { styled } from "@linaria/react";

import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP,
  PIPELINE_CANVAS_OVERLAY_Z_INDEX,
} from "@/pages/pipelines/canvas/constants";
import PipelineCanvasConnectionSelector from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelector";
import PipelineCanvasEditWidgetButton from "@/pages/pipelines/canvas/PipelineCanvasEditWidgetButton";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/providers/canvas/types";

const PipelineCanvasEditWidgetContainer = withTheme(styled.div<PropsWithTheme>`
  position: absolute;
  left: 16px;
  top: 50%;
  transform: translateY(-50%);
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  display: flex;
  padding: 8px;

  background-color: ${({ theme }) => theme.color.background.base};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 200px;
`);

const PipelineCanvasEditWidget = () => {
  const state = usePipelineCanvasState();
  const { setActiveMode } = usePipelineCanvasActions();

  const handleModeToggle = (mode: PipelineCanvasEditMode) => {
    setActiveMode(state.activeMode === mode ? null : mode);
  };

  const handleDropdownClose = () => {
    setActiveMode(null);
  };

  return (
    <PipelineCanvasEditWidgetContainer>
      <Dropdown
        position={DropdownPosition.RIGHT_START}
        isOpen={state.activeMode === PipelineCanvasEditMode.ADD_NODE}
        onClose={handleDropdownClose}
        body={<PipelineCanvasConnectionSelector />}
        offset={[-8, 16]}
        noPadding
      >
        <PipelineCanvasEditWidgetButton
          icon={PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP[PipelineCanvasEditMode.ADD_NODE]}
          isActive={state.activeMode === PipelineCanvasEditMode.ADD_NODE}
          onClick={() => handleModeToggle(PipelineCanvasEditMode.ADD_NODE)}
        />
      </Dropdown>
    </PipelineCanvasEditWidgetContainer>
  );
};

export default PipelineCanvasEditWidget;
