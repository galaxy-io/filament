import type { FC } from "react";

import { CaretDownIcon, CaretUpIcon, PulseIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import { SquareSize } from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import { usePipelineCanvasIsRunning } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasIsRunning";
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";

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
        <Button
          key="toggle"
          icon={isOpen ? CaretDownIcon : CaretUpIcon}
          ariaLabel={isOpen ? "Collapse activity" : "Expand activity"}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={onToggle}
        />,
      ]}
    />
  );
};

export default PipelineCanvasPanelActivityHeader;
