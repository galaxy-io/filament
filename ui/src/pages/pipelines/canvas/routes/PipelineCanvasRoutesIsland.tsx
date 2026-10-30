import type { FC, PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { FOCUS_RING, HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { PIPELINE_CANVAS_ROUTES_ISLAND_HEIGHT } from "@/pages/pipelines/canvas/routes/constants";
import { getPipelineCanvasRoutesActivateHandler } from "@/pages/pipelines/canvas/routes/utils";

const Island = styled.div<{ $width: number; $minWidth: number; $isSelected: boolean }>`
  flex: 0 1 ${({ $width }) => $width}px;
  min-width: ${({ $minWidth }) => $minWidth}px;
  height: ${PIPELINE_CANVAS_ROUTES_ISLAND_HEIGHT}px;
  padding: 0 ${t.space[8]} 0 ${t.space[4]};

  display: flex;
  align-items: center;
  gap: ${t.space[8]};

  background-color: ${t.color.background.secondary};
  border: ${HAIRLINE_WIDTH} solid
    ${({ $isSelected }) =>
      $isSelected ? t.color.solid.primary.background : t.color.border.primary};
  border-radius: ${t.radius.lg};
  outline: ${({ $isSelected }) =>
    $isSelected ? `${HAIRLINE_WIDTH} solid ${t.color.solid.primary.background}` : "none"};
  outline-offset: -${HAIRLINE_WIDTH};

  cursor: pointer;
  transition: border-color ${t.duration.fast};

  &:hover:not([data-selected="true"]) {
    border-color: ${t.color.border.hovered};
  }

  ${FOCUS_RING}
`;

interface PipelineCanvasRoutesIslandProps extends PropsWithChildren {
  width: number;
  minWidth: number;
  isSelected: boolean;
  onSelect: () => void;
}

const PipelineCanvasRoutesIsland: FC<PipelineCanvasRoutesIslandProps> = ({
  width,
  minWidth,
  isSelected,
  onSelect,
  children,
}) => {
  const handleClick = (event: React.MouseEvent) => {
    event.stopPropagation();
    onSelect();
  };

  return (
    <Island
      $width={width}
      $minWidth={minWidth}
      $isSelected={isSelected}
      data-island
      data-selected={isSelected}
      role="button"
      tabIndex={0}
      onClick={handleClick}
      onKeyDown={getPipelineCanvasRoutesActivateHandler(onSelect)}
    >
      {children}
    </Island>
  );
};

export default PipelineCanvasRoutesIsland;
