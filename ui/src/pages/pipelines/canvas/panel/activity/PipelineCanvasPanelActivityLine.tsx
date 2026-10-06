import { memo } from "react";

import { styled } from "@linaria/react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { RunEvent } from "@/gen/ingestion/v1/runs_pb";

import {
  formatRunEventDetail,
  formatRunEventTime,
  getRunEventTextVariant,
} from "@/pages/pipelines/canvas/panel/activity/utils";

const LineWrapper = styled.div`
  display: flex;
  align-items: baseline;
  gap: 8px;
`;

const Timestamp = styled.span`
  flex-shrink: 0;
  color: ${t.color.text.tertiary};
  font-family: inherit;
`;

const Detail = styled.div`
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
`;

interface PipelineCanvasPanelActivityLineProps {
  event: RunEvent;
}

const PipelineCanvasPanelActivityLine = memo(({ event }: PipelineCanvasPanelActivityLineProps) => {
  const detail = formatRunEventDetail(event);

  return (
    <LineWrapper>
      <Text size={TextSize.CAPTION} family={FontFamily.MONO}>
        <Timestamp>{formatRunEventTime(event)}</Timestamp>
      </Text>
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
    </LineWrapper>
  );
});

export default PipelineCanvasPanelActivityLine;
