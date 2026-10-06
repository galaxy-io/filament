import { styled } from "@linaria/react";
import { ArrowCounterClockwiseIcon, MinusIcon, PlusIcon } from "@phosphor-icons/react";
import { getViewportForBounds, useReactFlow, useStore } from "@xyflow/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_CANVAS_FIT_MAX_ZOOM,
  PIPELINE_CANVAS_FIT_MIN_ZOOM,
  PIPELINE_CANVAS_OVERLAY_Z_INDEX,
} from "@/pages/pipelines/canvas/constants";
import {
  getGraphBounds,
  getPlaceholderNodes,
  mapNodesToStackedPositions,
} from "@/pages/pipelines/canvas/graph/layout";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { getPipelineCanvasFitPadding } from "@/pages/pipelines/canvas/utils";

const ControlsContainer = styled.div`
  position: absolute;
  bottom: 16px;
  left: 16px;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  display: flex;
  flex-direction: column;

  background-color: ${t.color.background.primary};
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  overflow: hidden;
`;

const ControlButton = styled.button`
  width: 28px;
  height: 30px;
  padding: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: transparent;
  border: none;
  border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  cursor: pointer;

  transition: background-color 100ms ease;

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;

const PipelineCanvasControls = () => {
  const { zoomIn, zoomOut, setViewport } = useReactFlow();
  const width = useStore((store) => store.width);
  const height = useStore((store) => store.height);
  const state = usePipelineCanvasState();
  const isReadOnly = usePipelineCanvasReadOnly();
  const { setNodes } = usePipelineCanvasActions();
  const { showPanel } = usePipelineCanvasSelection();

  const handleResetView = () => {
    let repositioned = state.nodes;
    if (!isReadOnly) {
      repositioned = mapNodesToStackedPositions(state.nodes);
      setNodes(repositioned);
    }

    const bounds = getGraphBounds([
      ...repositioned,
      ...getPlaceholderNodes(repositioned, isReadOnly),
    ]);
    if (bounds.width === 0) return;

    void setViewport(
      getViewportForBounds(
        bounds,
        width,
        height,
        PIPELINE_CANVAS_FIT_MIN_ZOOM,
        PIPELINE_CANVAS_FIT_MAX_ZOOM,
        getPipelineCanvasFitPadding(showPanel),
      ),
    );
  };

  return (
    <ControlsContainer>
      <ControlButton onClick={() => zoomIn()}>
        <Icon component={PlusIcon} size={12} variant={IconVariant.SECONDARY} />
      </ControlButton>
      <ControlButton onClick={() => zoomOut()}>
        <Icon component={MinusIcon} size={12} variant={IconVariant.SECONDARY} />
      </ControlButton>
      <ControlButton onClick={handleResetView}>
        <Icon component={ArrowCounterClockwiseIcon} size={12} variant={IconVariant.SECONDARY} />
      </ControlButton>
    </ControlsContainer>
  );
};

export default PipelineCanvasControls;
