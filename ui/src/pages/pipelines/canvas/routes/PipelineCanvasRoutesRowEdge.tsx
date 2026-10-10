import type { FC } from "react";

import { styled } from "@linaria/react";
import { FunctionIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import { FOCUS_RING } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP,
  PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_INSET,
  PIPELINE_CANVAS_ROUTES_EDGE_HIT_PADDING,
  PIPELINE_CANVAS_ROUTES_EDGE_MIN_WIDTH,
  PIPELINE_CANVAS_ROUTES_ROW_HEIGHT,
} from "@/pages/pipelines/canvas/routes/constants";
import PipelineCanvasRoutesEdgeSvg from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesEdgeSvg";
import PipelineCanvasRoutesRowControls from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesRowControls";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";
import {
  getPipelineCanvasRoutesActivateHandler,
  getPipelineCanvasRouteTransformLabel,
} from "@/pages/pipelines/canvas/routes/utils";

const EDGE_HIT_HEIGHT = 1 + PIPELINE_CANVAS_ROUTES_EDGE_HIT_PADDING * 2;

const EdgeArea = styled.div`
  position: relative;
  flex: 1;
  min-width: ${PIPELINE_CANVAS_ROUTES_EDGE_MIN_WIDTH}px;
  height: 100%;
  padding: 0 ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_INSET}px;

  display: flex;
  align-items: center;
  gap: ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;

  &:has([data-edge-hit]:hover) [data-edge]:not([data-selected="true"]) line,
  &:has([data-edge-hit]:hover) [data-edge]:not([data-selected="true"]) path {
    stroke: ${t.color.border.hovered};
  }
`;

const EdgeHitArea = styled.div`
  position: absolute;
  left: 0;
  right: 0;
  top: ${PIPELINE_CANVAS_ROUTES_ROW_HEIGHT / 2 - EDGE_HIT_HEIGHT / 2}px;
  height: ${EDGE_HIT_HEIGHT}px;

  cursor: pointer;
  ${FOCUS_RING}
`;

interface PipelineCanvasRoutesRowEdgeProps {
  route: PipelineCanvasRoute;
  isSelected: boolean;
  isRunning: boolean;
  onSelect: () => void;
}

const PipelineCanvasRoutesRowEdge: FC<PipelineCanvasRoutesRowEdgeProps> = ({
  route,
  isSelected,
  isRunning,
  onSelect,
}) => {
  const handleClick = (event: React.MouseEvent) => {
    event.stopPropagation();
    onSelect();
  };

  return (
    <EdgeArea>
      <PipelineCanvasRoutesEdgeSvg
        groupIndex={route.groupIndex}
        isSelected={isSelected}
        isRunning={isRunning}
      />
      <EdgeHitArea
        data-edge-hit
        role="button"
        tabIndex={0}
        aria-label="Select route"
        onClick={handleClick}
        onKeyDown={getPipelineCanvasRoutesActivateHandler(onSelect)}
      />
      <PipelineCanvasRoutesRowControls route={route}>
        {route.transformStepCount > 0 && (
          <Chip
            label={getPipelineCanvasRouteTransformLabel(route.transformStepCount)}
            icon={FunctionIcon}
            color="blue"
            size={ChipSize.SMALL}
            isPill
          />
        )}
      </PipelineCanvasRoutesRowControls>
    </EdgeArea>
  );
};

export default PipelineCanvasRoutesRowEdge;
