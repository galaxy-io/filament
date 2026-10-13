import { type FC, useMemo, useRef, useState } from "react";

import { styled } from "@linaria/react";
import { PulseIcon } from "@phosphor-icons/react";
import { useVirtualizer } from "@tanstack/react-virtual";

import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Flasher from "@galaxy-io/dls/transform/Flasher";

import type { RunEvent, RunInfo } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_CANVAS_PANEL_ACTIVITY_ESTIMATED_LINE_HEIGHT,
  PIPELINE_CANVAS_PANEL_ACTIVITY_LINE_GAP,
} from "@/pages/pipelines/canvas/panel/activity/constants";
import { usePipelineCanvasPanelActivityStickToBottom } from "@/pages/pipelines/canvas/panel/activity/hooks/usePipelineCanvasPanelActivityStickToBottom";
import PipelineCanvasPanelActivityLine from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivityLine";
import {
  appendUnseenRunEvents,
  getRunEventKey,
} from "@/pages/pipelines/canvas/panel/activity/utils";

import { useTailRunsStream } from "@/api/queries/runs";

const ActivityList = styled.div`
  position: relative;
  flex-shrink: 0;
  width: 100%;
`;

const ActivityListLine = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
`;

interface PipelineCanvasPanelActivityProps {
  runIds: RunInfo["id"][];
}

const PipelineCanvasPanelActivity: FC<PipelineCanvasPanelActivityProps> = ({ runIds }) => {
  const { events, isStreaming } = useTailRunsStream(runIds);

  const bodyRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLElement>(null);
  const isPinned = usePipelineCanvasPanelActivityStickToBottom(
    bodyRef,
    contentRef,
    runIds.length > 0,
  );

  const [frozenEvents, setFrozenEvents] = useState<RunEvent[] | null>(null);
  if (!isPinned && frozenEvents === null) {
    setFrozenEvents(events);
  }
  if (isPinned && frozenEvents !== null) {
    setFrozenEvents(null);
  }
  const shownEvents = useMemo(
    () => (frozenEvents ? appendUnseenRunEvents(frozenEvents, events) : events),
    [frozenEvents, events],
  );

  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: shownEvents.length,
    getScrollElement: () => bodyRef.current,
    estimateSize: () => PIPELINE_CANVAS_PANEL_ACTIVITY_ESTIMATED_LINE_HEIGHT,
    getItemKey: (index) => getRunEventKey(shownEvents[index]),
    gap: PIPELINE_CANVAS_PANEL_ACTIVITY_LINE_GAP,
  });

  if (runIds.length === 0) {
    return (
      <Flex direction={FlexDirection.COLUMN} grow={1} minHeight={0} padding={[8, 12]}>
        <EmptyLayout icon={PulseIcon} header="Run the pipeline to see activity" />
      </Flex>
    );
  }

  return (
    <FlexItem grow={1} minHeight={0}>
      <ScrollArea ref={bodyRef}>
        <Flex
          ref={contentRef}
          direction={FlexDirection.COLUMN}
          gap={PIPELINE_CANVAS_PANEL_ACTIVITY_LINE_GAP}
          padding={[8, 12]}
        >
          <ActivityList style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((item) => (
              <ActivityListLine
                key={item.key}
                ref={virtualizer.measureElement}
                data-index={item.index}
                style={{ transform: `translateY(${item.start}px)` }}
              >
                <PipelineCanvasPanelActivityLine event={shownEvents[item.index]} />
              </ActivityListLine>
            ))}
          </ActivityList>
          {isStreaming && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} family={FontFamily.MONO}>
              <Flasher isFlashing>Listening...</Flasher>
            </Text>
          )}
        </Flex>
      </ScrollArea>
    </FlexItem>
  );
};

export default PipelineCanvasPanelActivity;
