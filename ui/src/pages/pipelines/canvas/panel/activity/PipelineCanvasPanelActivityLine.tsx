import { memo } from "react";

import { styled } from "@linaria/react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

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

const Timestamp = withTheme(styled.span<PropsWithTheme>`
  flex-shrink: 0;
  color: ${({ theme }) => theme.color.text.tertiary};
  font-family: inherit;
`);

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
      <Text size={TextSize.CAPTION} isMonospace>
        <Timestamp>{formatRunEventTime(event)}</Timestamp>
      </Text>
      <Detail>
        <Text
          size={TextSize.CAPTION}
          variant={getRunEventTextVariant(event)}
          isMonospace
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
