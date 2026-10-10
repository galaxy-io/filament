import { type FC, memo } from "react";

import { styled } from "@linaria/react";

import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { createNodeFromConnection } from "@/pages/pipelines/canvas/graph/rules";
import {
  CONNECTOR_KIND_TO_PLACEHOLDER_DESCRIPTION_MAP,
  CONNECTOR_KIND_TO_PLACEHOLDER_TITLE_MAP,
  PIPELINE_CANVAS_NODE_GAP,
  PIPELINE_CANVAS_NODE_PADDING,
  PIPELINE_CANVAS_NODE_PLACEHOLDER_PADDING_Y,
  PIPELINE_CANVAS_NODE_PLACEHOLDER_SELECTOR_HEIGHT,
  PIPELINE_CANVAS_NODE_WIDTH,
} from "@/pages/pipelines/canvas/nodes/constants";
import type { PipelineCanvasNodePlaceholderProps } from "@/pages/pipelines/canvas/nodes/types";
import PipelineCanvasConnectionSelector from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelector";
import { usePipelineCanvasActions } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const PlaceholderCard = styled.div`
  padding: ${PIPELINE_CANVAS_NODE_PLACEHOLDER_PADDING_Y}px ${t.space[16]};

  display: flex;
  flex-direction: column;
  gap: ${PIPELINE_CANVAS_NODE_GAP}px;

  background-color: ${t.color.background.base};
  border: ${HAIRLINE_WIDTH} dashed ${t.color.border.primary};
  border-radius: ${t.radius.lg};
`;

const SelectorIsland = styled.div`
  height: ${PIPELINE_CANVAS_NODE_PLACEHOLDER_SELECTOR_HEIGHT}px;

  background-color: ${t.color.background.primary};
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  overflow: hidden;

  transition: border-color 100ms ease;

  &:hover,
  &:focus-within {
    border-color: ${t.color.border.tertiary};
  }
`;

const PipelineCanvasNodePlaceholder: FC<PipelineCanvasNodePlaceholderProps> = ({
  data,
  positionAbsoluteX,
  positionAbsoluteY,
}) => {
  const { addNode } = usePipelineCanvasActions();

  const handleSelect = (connection: Connection) => {
    addNode(createNodeFromConnection(connection, { x: positionAbsoluteX, y: positionAbsoluteY }));
  };

  return (
    <PlaceholderCard>
      <Flex direction={FlexDirection.COLUMN} gap={2}>
        <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
          {CONNECTOR_KIND_TO_PLACEHOLDER_TITLE_MAP[data.kind]}
        </Text>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          {CONNECTOR_KIND_TO_PLACEHOLDER_DESCRIPTION_MAP[data.kind]}
        </Text>
      </Flex>
      <SelectorIsland className="nodrag nowheel">
        <PipelineCanvasConnectionSelector
          kindFilter={data.kind}
          width={PIPELINE_CANVAS_NODE_WIDTH - PIPELINE_CANVAS_NODE_PADDING * 2}
          onSelect={handleSelect}
          fillHeight
        />
      </SelectorIsland>
    </PlaceholderCard>
  );
};

export default memo(PipelineCanvasNodePlaceholder);
