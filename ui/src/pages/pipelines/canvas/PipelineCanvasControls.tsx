import { styled } from "@linaria/react";
import { ArrowCounterClockwiseIcon, MinusIcon, PlusIcon } from "@phosphor-icons/react";
import { getViewportForBounds, useReactFlow, useStore } from "@xyflow/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_CANVAS_FIT_MAX_ZOOM,
  PIPELINE_CANVAS_FIT_MIN_ZOOM,
  PIPELINE_CANVAS_FIT_PADDING,
  PIPELINE_CANVAS_OVERLAY_Z_INDEX,
} from "@/pages/pipelines/canvas/constants";
import {
  getGraphBounds,
  getPlaceholderNodes,
  mapNodesToStackedPositions,
} from "@/pages/pipelines/canvas/graph/layout";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const ControlsContainer = withTheme(styled.div<PropsWithTheme>`
  position: absolute;
  bottom: 16px;
  left: 16px;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;
  overflow: hidden;
`);

const ControlButton = withTheme(styled.button<PropsWithTheme>`
  width: 28px;
  height: 30px;
  padding: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: transparent;
  border: none;
  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};
  cursor: pointer;

  transition: background-color 100ms ease;

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }
`);

const PipelineCanvasControls = () => {
  const { zoomIn, zoomOut, setViewport } = useReactFlow();
  const width = useStore((store) => store.width);
  const height = useStore((store) => store.height);
  const state = usePipelineCanvasState();
  const isReadOnly = usePipelineCanvasReadOnly();
  const { setNodes } = usePipelineCanvasActions();

  const handleResetView = () => {
    const repositioned = mapNodesToStackedPositions(state.nodes);
    setNodes(repositioned);

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
        PIPELINE_CANVAS_FIT_PADDING,
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
