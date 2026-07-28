import { memo } from "react";

import { styled } from "@linaria/react";

import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import {
  PIPELINE_NODE_SINK_HANDLE_ID,
  PIPELINE_NODE_SOURCE_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import { createNodeFromConnection } from "@/pages/pipelines/canvas/graph";
import {
  PIPELINE_NODE_BORDER_RADIUS,
  PIPELINE_NODE_GAP,
  PIPELINE_NODE_PADDING,
  PIPELINE_NODE_PLACEHOLDER_SELECTOR_HEIGHT,
  PIPELINE_NODE_WIDTH,
} from "@/pages/pipelines/canvas/nodes/constants";
import type { PipelineNodePlaceholderProps } from "@/pages/pipelines/canvas/nodes/types";
import PipelineCanvasConnectionSelector from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelector";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import { PipelineNodeType } from "@/pages/pipelines/canvas/types";

const PlaceholderCard = withTheme(styled.div<PropsWithTheme>`
  padding: 20px 16px;

  display: flex;
  flex-direction: column;
  gap: ${PIPELINE_NODE_GAP}px;

  background-color: ${({ theme }) => theme.color.background.base};
  border: 1px dashed ${({ theme }) => theme.color.border.primary};
  border-radius: ${PIPELINE_NODE_BORDER_RADIUS}px;
`);

const CardHeader = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const SelectorIsland = withTheme(styled.div<PropsWithTheme>`
  height: ${PIPELINE_NODE_PLACEHOLDER_SELECTOR_HEIGHT}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: ${PIPELINE_NODE_BORDER_RADIUS}px;
  overflow: hidden;

  transition: border-color 100ms ease;

  &:hover,
  &:focus-within {
    border-color: ${({ theme }) => theme.color.border.tertiary};
  }
`);

const PipelineNodePlaceholder = memo(
  ({ data, positionAbsoluteX, positionAbsoluteY }: PipelineNodePlaceholderProps) => {
    const { state, dispatch } = usePipelineCanvas();

    const isSource = data.kind === ConnectorKind.SOURCE;

    const handleSelect = (connection: Connection) => {
      const node = createNodeFromConnection(connection, {
        x: positionAbsoluteX,
        y: positionAbsoluteY,
      });

      dispatch({ type: PipelineCanvasActionType.ADD_NODE, payload: node });

      const counterpartType = isSource ? PipelineNodeType.SINK : PipelineNodeType.SOURCE;
      for (const counterpart of state.nodes.filter((n) => n.type === counterpartType)) {
        dispatch({
          type: PipelineCanvasActionType.CONNECT,
          payload: {
            source: isSource ? node.id : counterpart.id,
            sourceHandle: PIPELINE_NODE_SOURCE_HANDLE_ID,
            target: isSource ? counterpart.id : node.id,
            targetHandle: PIPELINE_NODE_SINK_HANDLE_ID,
          },
        });
      }
    };

    return (
      <PlaceholderCard>
        <CardHeader>
          <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
            {isSource ? "Select a source" : "Select a sink"}
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            {isSource ? "A source is where data comes from." : "A sink is where data goes to."}
          </Text>
        </CardHeader>
        <SelectorIsland className="nodrag nowheel">
          <PipelineCanvasConnectionSelector
            kindFilter={data.kind}
            width={PIPELINE_NODE_WIDTH - PIPELINE_NODE_PADDING * 2}
            onSelect={handleSelect}
            fillHeight
          />
        </SelectorIsland>
      </PlaceholderCard>
    );
  },
);

export default PipelineNodePlaceholder;
