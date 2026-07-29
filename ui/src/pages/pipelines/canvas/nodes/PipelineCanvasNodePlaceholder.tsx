import { memo } from "react";

import { styled } from "@linaria/react";

import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { createNodeFromConnection } from "@/pages/pipelines/canvas/graph/rules";
import {
  CONNECTOR_KIND_TO_PLACEHOLDER_DESCRIPTION_MAP,
  CONNECTOR_KIND_TO_PLACEHOLDER_TITLE_MAP,
  PIPELINE_CANVAS_NODE_BORDER_RADIUS,
  PIPELINE_CANVAS_NODE_GAP,
  PIPELINE_CANVAS_NODE_PADDING,
  PIPELINE_CANVAS_NODE_PLACEHOLDER_SELECTOR_HEIGHT,
  PIPELINE_CANVAS_NODE_WIDTH,
} from "@/pages/pipelines/canvas/nodes/constants";
import type { PipelineCanvasNodePlaceholderProps } from "@/pages/pipelines/canvas/nodes/types";
import PipelineCanvasConnectionSelector from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelector";
import { usePipelineCanvasActions } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const PlaceholderCard = withTheme(styled.div<PropsWithTheme>`
  padding: 20px 16px;

  display: flex;
  flex-direction: column;
  gap: ${PIPELINE_CANVAS_NODE_GAP}px;

  background-color: ${({ theme }) => theme.color.background.base};
  border: 1px dashed ${({ theme }) => theme.color.border.primary};
  border-radius: ${PIPELINE_CANVAS_NODE_BORDER_RADIUS}px;
`);

const CardHeader = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const SelectorIsland = withTheme(styled.div<PropsWithTheme>`
  height: ${PIPELINE_CANVAS_NODE_PLACEHOLDER_SELECTOR_HEIGHT}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: ${PIPELINE_CANVAS_NODE_BORDER_RADIUS}px;
  overflow: hidden;

  transition: border-color 100ms ease;

  &:hover,
  &:focus-within {
    border-color: ${({ theme }) => theme.color.border.tertiary};
  }
`);

const PipelineCanvasNodePlaceholder = memo(
  ({ data, positionAbsoluteX, positionAbsoluteY }: PipelineCanvasNodePlaceholderProps) => {
    const { addNode } = usePipelineCanvasActions();

    const handleSelect = (connection: Connection) => {
      addNode(createNodeFromConnection(connection, { x: positionAbsoluteX, y: positionAbsoluteY }));
    };

    return (
      <PlaceholderCard>
        <CardHeader>
          <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
            {CONNECTOR_KIND_TO_PLACEHOLDER_TITLE_MAP[data.kind]}
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            {CONNECTOR_KIND_TO_PLACEHOLDER_DESCRIPTION_MAP[data.kind]}
          </Text>
        </CardHeader>
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
  },
);

export default PipelineCanvasNodePlaceholder;
