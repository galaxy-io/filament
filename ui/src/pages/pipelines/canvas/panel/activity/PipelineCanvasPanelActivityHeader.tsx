import type { FC } from "react";

import { styled } from "@linaria/react";
import { PulseIcon } from "@phosphor-icons/react";

import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import { SquareSize } from "@galaxy-io/dls/shapes/Square";
import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import { usePipelineCanvasIsRunning } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasIsRunning";
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";

const ActivityHeaderToggle = styled.button`
  ${INTERACTIVE_RESET}
  position: absolute;
  inset: 0;
  cursor: pointer;
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }

  ${FOCUS_RING}
`;

const ActivityHeaderContent = styled.div`
  position: relative;
  pointer-events: none;
`;

interface PipelineCanvasPanelActivityHeaderProps {
  isOpen: boolean;
  onToggle: () => void;
}

const PipelineCanvasPanelActivityHeader: FC<PipelineCanvasPanelActivityHeaderProps> = ({
  isOpen,
  onToggle,
}) => {
  const isRunning = usePipelineCanvasIsRunning();

  return (
    <Box position="relative">
      <ActivityHeaderToggle
        type="button"
        aria-expanded={isOpen}
        aria-label={isOpen ? "Collapse activity" : "Expand activity"}
        onClick={onToggle}
      />
      <ActivityHeaderContent>
        <PipelineCanvasPanelHeader
          title="Activity"
          icon={PulseIcon}
          hasDivider={isOpen}
          actions={[
            isRunning && (
              <Flex key="running" alignItems={AlignItems.CENTER} gap={8} shrink={0}>
                <PipelineRunStatusSwatch status={RunStatus.RUNNING} size={SquareSize.SMALL} />
                <Text as="span" size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Running
                </Text>
              </Flex>
            ),
          ]}
        />
      </ActivityHeaderContent>
    </Box>
  );
};

export default PipelineCanvasPanelActivityHeader;
