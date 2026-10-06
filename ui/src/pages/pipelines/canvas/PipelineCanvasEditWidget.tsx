import { styled } from "@linaria/react";

import Dropdown from "@galaxy-io/dls/dropdown/Dropdown";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

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

const PipelineCanvasEditWidgetContainer = styled.div`
  position: absolute;
  left: 16px;
  top: 50%;
  transform: translateY(-50%);
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  display: flex;
  padding: 8px;

  background-color: ${t.color.background.base};
  border: 0.5px solid ${t.color.border.primary};
  border-radius: 200px;
`;

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
        placement={Placement.RIGHT_START}
        isOpen={state.activeMode === PipelineCanvasEditMode.ADD_NODE}
        /* @dls-migrate dropdown.onClose: A controlled 2.0 Dropdown also asks to open from its trigger: switch to `onOpenChange` and remove the trigger's own toggle. */ onClose={
          handleDropdownClose
        }
        body={<PipelineCanvasConnectionSelector />}
        /* @dls-migrate dropdown.noPadding: A panel that hosts its own layout is a `Popover`. */ noPadding
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
