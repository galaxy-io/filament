import { type FC, memo } from "react";

import { styled } from "@linaria/react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { RunEvent } from "@/gen/ingestion/v1/runs_pb";

import {
  formatRunEventDetail,
  formatRunEventTime,
  getRunEventTextVariant,
} from "@/pages/pipelines/canvas/panel/activity/utils";

const Detail = styled.div`
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
`;

interface PipelineCanvasPanelActivityLineProps {
  event: RunEvent;
}

const PipelineCanvasPanelActivityLine: FC<PipelineCanvasPanelActivityLineProps> = ({ event }) => {
  const detail = formatRunEventDetail(event);

  return (
    <Flex alignItems={AlignItems.BASELINE} gap={8}>
      <FlexItem shrink={0}>
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} family={FontFamily.MONO}>
          {formatRunEventTime(event)}
        </Text>
      </FlexItem>
      <Detail>
        <Text
          size={TextSize.CAPTION}
          variant={getRunEventTextVariant(event)}
          family={FontFamily.MONO}
          isSelectable
        >
          {event.eventType}
          {detail ? ` ${detail}` : ""}
        </Text>
      </Detail>
    </Flex>
  );
};

export default memo(PipelineCanvasPanelActivityLine);
