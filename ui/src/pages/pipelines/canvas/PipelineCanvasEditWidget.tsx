import type { FC } from "react";

import { styled } from "@linaria/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Popover, { PopoverVariant } from "@galaxy-io/dls/overlays/Popover";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP,
  PIPELINE_CANVAS_OVERLAY_Z_INDEX,
} from "@/pages/pipelines/canvas/constants";
import PipelineCanvasConnectionSelector from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelector";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/providers/canvas/types";

const PipelineCanvasEditWidgetContainer = styled.div`
  position: absolute;
  left: ${t.space[16]};
  top: 50%;
  transform: translateY(-50%);
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  display: flex;
  padding: ${t.space[8]};

  background-color: ${t.color.background.base};
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.pill};
`;

const PipelineCanvasEditWidget: FC = () => {
  const state = usePipelineCanvasState();
  const { setActiveMode } = usePipelineCanvasActions();

  const isAddNodeOpen = state.activeMode === PipelineCanvasEditMode.ADD_NODE;

  const handleAddNodeOpenChange = (isOpen: boolean) => {
    setActiveMode(isOpen ? PipelineCanvasEditMode.ADD_NODE : null);
  };

  return (
    <PipelineCanvasEditWidgetContainer>
      <Popover
        placement={Placement.RIGHT_START}
        variant={PopoverVariant.PRIMARY}
        isOpen={isAddNodeOpen}
        onOpenChange={handleAddNodeOpenChange}
        body={<PipelineCanvasConnectionSelector />}
      >
        <Button
          icon={PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP[PipelineCanvasEditMode.ADD_NODE]}
          variant={ButtonVariant.PRIMARY}
          isRound
          isActive={isAddNodeOpen}
          ariaLabel="Add a connection"
        />
      </Popover>
    </PipelineCanvasEditWidgetContainer>
  );
};

export default PipelineCanvasEditWidget;
