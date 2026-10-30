import { type FC, memo } from "react";

import { styled } from "@linaria/react";

import Flex from "@galaxy-io/dls/layout/Flex";

import {
  PIPELINE_CANVAS_ROUTES_LIST_PADDING_X,
  PIPELINE_CANVAS_ROUTES_ROW_HEIGHT,
  PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_MIN_WIDTH,
  PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_WIDTH,
} from "@/pages/pipelines/canvas/routes/constants";
import PipelineCanvasRoutesRowEdge from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesRowEdge";
import PipelineCanvasRoutesSinkIsland from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesSinkIsland";
import PipelineCanvasRoutesSourceIsland from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesSourceIsland";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

const RowWrapper = styled.div`
  height: ${PIPELINE_CANVAS_ROUTES_ROW_HEIGHT}px;
  padding: 0 ${PIPELINE_CANVAS_ROUTES_LIST_PADDING_X}px;

  display: flex;
  align-items: center;
`;

interface PipelineCanvasRoutesRowProps {
  route: PipelineCanvasRoute;
  isSelected: boolean;
  isGroupSelected: boolean;
  isRunning: boolean;
  onSelect: (edgeId: CanvasEdge["id"]) => void;
  onAddSink: ((resource: PipelineCanvasRoute["resource"]) => void) | undefined;
}

const PipelineCanvasRoutesRow: FC<PipelineCanvasRoutesRowProps> = ({
  route,
  isSelected,
  isGroupSelected,
  isRunning,
  onSelect,
  onAddSink,
}) => (
  <RowWrapper>
    <Flex
      basis={PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_WIDTH}
      minWidth={PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_MIN_WIDTH}
    >
      {route.groupIndex === 0 && (
        <PipelineCanvasRoutesSourceIsland
          route={route}
          isSelected={isGroupSelected}
          onSelect={() => onSelect(route.edge.id)}
          onAddSink={onAddSink && (() => onAddSink(route.resource))}
        />
      )}
    </Flex>
    <PipelineCanvasRoutesRowEdge
      route={route}
      isSelected={isSelected}
      isRunning={isRunning}
      onSelect={() => onSelect(route.edge.id)}
    />
    <PipelineCanvasRoutesSinkIsland
      route={route}
      isSelected={isSelected}
      onSelect={() => onSelect(route.edge.id)}
    />
  </RowWrapper>
);

export default memo(PipelineCanvasRoutesRow);
