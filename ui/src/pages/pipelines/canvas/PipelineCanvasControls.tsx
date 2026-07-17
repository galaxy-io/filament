import { styled } from "@linaria/react";
import { CornersOutIcon, MinusIcon, PlusIcon } from "@phosphor-icons/react";
import { useReactFlow } from "@xyflow/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  CANVAS_FIT_VIEW_OPTIONS,
  CANVAS_FIT_VIEW_Y_OFFSET,
} from "@/pages/pipelines/canvas/constants";

const ControlsContainer = withTheme(styled.div<PropsWithTheme>`
  position: absolute;
  bottom: 16px;
  left: 16px;
  z-index: 1001;

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
  const { zoomIn, zoomOut, fitView, getViewport, setViewport } = useReactFlow();

  const handleFitView = async () => {
    await fitView(CANVAS_FIT_VIEW_OPTIONS);
    const viewport = getViewport();
    setViewport({
      ...viewport,
      y: viewport.y - CANVAS_FIT_VIEW_Y_OFFSET,
    });
  };

  return (
    <ControlsContainer>
      <ControlButton onClick={() => zoomIn()}>
        <Icon component={PlusIcon} size={12} variant={IconVariant.SECONDARY} />
      </ControlButton>
      <ControlButton onClick={() => zoomOut()}>
        <Icon component={MinusIcon} size={12} variant={IconVariant.SECONDARY} />
      </ControlButton>
      <ControlButton onClick={handleFitView}>
        <Icon component={CornersOutIcon} size={12} variant={IconVariant.SECONDARY} />
      </ControlButton>
    </ControlsContainer>
  );
};

export default PipelineCanvasControls;
