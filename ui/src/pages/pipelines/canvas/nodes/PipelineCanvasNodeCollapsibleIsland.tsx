import { type PropsWithChildren, useCallback, useEffect, useState } from "react";

import { styled } from "@linaria/react";
import { CaretRightIcon } from "@phosphor-icons/react";
import { useNodeId, useUpdateNodeInternals } from "@xyflow/react";
import { match } from "ts-pattern";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Rotate from "@galaxy-io/dls/transform/Rotate";

import {
  PIPELINE_CANVAS_NODE_ISLAND_BODY_MAX_HEIGHT,
  PIPELINE_CANVAS_NODE_PADDING,
} from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeIsland";

const COLLAPSE_PROPERTY = "grid-template-rows";
const CARET_ROTATION_DEG = 90;
const CARET_BUTTON_SIZE = 24;
const CARET_BUTTON_BORDER_RADIUS = 5;

const IslandWrapper = styled(PipelineCanvasNodeIsland)`
  padding: 0;
`;

const IslandHeader = styled.div`
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px;
  cursor: pointer;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
`;

const IslandHeaderLeading = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
`;

const CaretButton = withTheme(styled.button<PropsWithTheme>`
  width: ${CARET_BUTTON_SIZE}px;
  height: ${CARET_BUTTON_SIZE}px;
  padding: 0;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.tertiary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: ${CARET_BUTTON_BORDER_RADIUS}px;
  cursor: pointer;

  transition: opacity 100ms ease;

  &:hover {
    opacity: 0.8;
  }
`);

const CollapseClip = styled.div<{ $isOpen: boolean }>`
  display: grid;
  grid-template-rows: ${({ $isOpen }) => ($isOpen ? "1fr" : "0fr")};

  transition: ${COLLAPSE_PROPERTY} 150ms ease;
`;

const CollapseInner = styled.div`
  overflow: hidden;
`;

const IslandBody = styled.div`
  max-height: ${PIPELINE_CANVAS_NODE_ISLAND_BODY_MAX_HEIGHT}px;
  overflow-y: auto;
`;

interface PipelineCanvasNodeCollapsibleIslandProps {
  title: string;
  isOpen: boolean;
  onToggle: () => void;
  trailing?: React.ReactNode;
  bodyHeader?: React.ReactNode;
  bodyRef?: React.Ref<HTMLDivElement>;
  isSelected?: boolean;
  onBodyScroll?: () => void;
}

interface PipelineCanvasNodeCollapsibleIslandState {
  isHovered: boolean;
}

const DEFAULT_STATE: PipelineCanvasNodeCollapsibleIslandState = {
  isHovered: false,
};

const PipelineCanvasNodeCollapsibleIsland = ({
  title,
  isOpen,
  onToggle,
  trailing,
  bodyHeader,
  bodyRef,
  isSelected,
  onBodyScroll,
  children,
}: PropsWithChildren<PipelineCanvasNodeCollapsibleIslandProps>) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const [state, setState] = useState<PipelineCanvasNodeCollapsibleIslandState>(DEFAULT_STATE);

  const syncNodeInternals = useCallback(() => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
  }, [nodeId, updateNodeInternals]);

  useEffect(() => {
    syncNodeInternals();
  }, [syncNodeInternals]);

  const handleToggle = useCallback(
    (event: React.MouseEvent) => {
      event.stopPropagation();
      onToggle();
    },
    [onToggle],
  );

  const handleTransitionEnd = useCallback(
    (event: React.TransitionEvent) => {
      if (event.target !== event.currentTarget || event.propertyName !== COLLAPSE_PROPERTY) {
        return;
      }
      syncNodeInternals();
    },
    [syncNodeInternals],
  );

  const handleMouseEnter = useCallback(() => {
    setState((prev) => ({ ...prev, isHovered: true }));
  }, []);

  const handleMouseLeave = useCallback(() => {
    setState((prev) => ({ ...prev, isHovered: false }));
  }, []);

  const titleVariant = match({ isOpen, isHovered: state.isHovered })
    .with({ isOpen: true }, () => TextVariant.PRIMARY)
    .with({ isHovered: true }, () => TextVariant.SECONDARY)
    .otherwise(() => TextVariant.TERTIARY);

  return (
    <IslandWrapper $isSelected={isSelected}>
      <IslandHeader
        className="nodrag"
        onClick={handleToggle}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <IslandHeaderLeading>
          <CaretButton onClick={handleToggle}>
            <Rotate isRotated={isOpen} deg={CARET_ROTATION_DEG}>
              <Icon component={CaretRightIcon} size={12} variant={IconVariant.SECONDARY} />
            </Rotate>
          </CaretButton>
          <Text size={TextSize.BODY_SM} variant={titleVariant}>
            {title}
          </Text>
        </IslandHeaderLeading>
        {trailing}
      </IslandHeader>

      <CollapseClip $isOpen={isOpen} onTransitionEnd={handleTransitionEnd}>
        <CollapseInner>
          <HorizontalDivider />
          {bodyHeader}
          {bodyHeader && <HorizontalDivider />}
          <IslandBody ref={bodyRef} className="nowheel" onScroll={onBodyScroll}>
            {children}
          </IslandBody>
        </CollapseInner>
      </CollapseClip>
    </IslandWrapper>
  );
};

export default PipelineCanvasNodeCollapsibleIsland;
