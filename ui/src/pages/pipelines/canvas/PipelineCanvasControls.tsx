import type { FC } from "react";

import { styled } from "@linaria/react";
import { ArrowCounterClockwiseIcon, MinusIcon, PlusIcon } from "@phosphor-icons/react";
import { getViewportForBounds, useReactFlow, useStore } from "@xyflow/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import ButtonGroup from "@galaxy-io/dls/buttons/ButtonGroup";
import { Orientation } from "@galaxy-io/dls/theme/enums";
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

const ControlsWrapper = styled.div`
  position: absolute;
  bottom: ${t.space[16]};
  left: ${t.space[16]};
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};
`;

const PipelineCanvasControls: FC = () => {
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
    <ControlsWrapper>
      <ButtonGroup isAttached orientation={Orientation.VERTICAL} ariaLabel="Canvas view">
        <Button
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          ariaLabel="Zoom in"
          onClick={() => zoomIn()}
        />
        <Button
          icon={MinusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          ariaLabel="Zoom out"
          onClick={() => zoomOut()}
        />
        <Button
          icon={ArrowCounterClockwiseIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          ariaLabel="Reset view"
          onClick={handleResetView}
        />
      </ButtonGroup>
    </ControlsWrapper>
  );
};

export default PipelineCanvasControls;
