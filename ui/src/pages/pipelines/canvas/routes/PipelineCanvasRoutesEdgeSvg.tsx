import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_CANVAS_ROUTES_EDGE_DASH_ARRAY,
  PIPELINE_CANVAS_ROUTES_EDGE_DASH_DURATION,
  PIPELINE_CANVAS_ROUTES_EDGE_DASH_PERIOD,
} from "@/pages/pipelines/canvas/routes/constants";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";
import { getPipelineCanvasRouteEdgeGeometry } from "@/pages/pipelines/canvas/routes/utils";

const EdgeSvg = styled.svg`
  position: absolute;
  inset: 0;
  z-index: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;

  line,
  path {
    fill: none;
    stroke: ${t.color.border.primary};
    stroke-width: 1;
    transition: stroke ${t.duration.fast};
  }

  &[data-selected="true"] line,
  &[data-selected="true"] path {
    stroke: ${t.color.solid.primary.background};
    stroke-width: 2;
  }

  &[data-running="true"] line,
  &[data-running="true"] path {
    stroke: ${t.color.solid.primary.background};
    stroke-width: 2;
    stroke-dasharray: ${PIPELINE_CANVAS_ROUTES_EDGE_DASH_ARRAY};
    animation: pipeline-canvas-table-edge-dash ${PIPELINE_CANVAS_ROUTES_EDGE_DASH_DURATION} linear
      infinite;
  }

  @keyframes pipeline-canvas-table-edge-dash {
    from {
      stroke-dashoffset: ${PIPELINE_CANVAS_ROUTES_EDGE_DASH_PERIOD};
    }
  }

  @media (prefers-reduced-motion: reduce) {
    &[data-running="true"] line,
    &[data-running="true"] path {
      animation: none;
    }
  }
`;

interface PipelineCanvasRoutesEdgeSvgProps {
  groupIndex: PipelineCanvasRoute["groupIndex"];
  isSelected: boolean;
  isRunning: boolean;
}

const PipelineCanvasRoutesEdgeSvg = ({
  groupIndex,
  isSelected,
  isRunning,
}: PipelineCanvasRoutesEdgeSvgProps) => {
  const { cy, lineStartX, fanPath } = getPipelineCanvasRouteEdgeGeometry(groupIndex);

  return (
    <EdgeSvg data-edge data-selected={isSelected} data-running={isRunning}>
      {fanPath && <path d={fanPath} />}
      <line x1={lineStartX} y1={cy} x2="100%" y2={cy} />
    </EdgeSvg>
  );
};

export default PipelineCanvasRoutesEdgeSvg;
