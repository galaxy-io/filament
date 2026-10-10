import { type FC, useRef, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { PulseIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";
import { useVirtualizer } from "@tanstack/react-virtual";

import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Flasher from "@galaxy-io/dls/transform/Flasher";

import { ListRunsRequestSchema, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import { PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS } from "@/pages/pipelines/canvas/panel/activity/constants";
import PipelineCanvasPanelActivityLine from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivityLine";
import { getRunEventKey } from "@/pages/pipelines/canvas/panel/activity/utils";

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useListRunsQuery, useTailRunsStream } from "@/api/queries/runs";

const ESTIMATED_LINE_HEIGHT = 18;
const LINE_GAP = 2;

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

const PipelineCanvasPanelActivity: FC = () => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data: activeRunsData } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
      pagination: { pageSize: PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS },
    }),
  });

  const [runIds, setRunIds] = useState<RunInfo["id"][]>([]);
  const mergedRunIds = [
    ...new Set([...runIds, ...(activeRunsData?.runs ?? []).map((run) => run.id)]),
  ].slice(-PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS);
  if (mergedRunIds.join("|") !== runIds.join("|")) {
    setRunIds(mergedRunIds);
  }

  const { events, isStreaming } = useTailRunsStream(runIds);

  const bodyRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: events.length,
    getScrollElement: () => bodyRef.current,
    estimateSize: () => ESTIMATED_LINE_HEIGHT,
    getItemKey: (index) => getRunEventKey(events[index]),
    gap: LINE_GAP,
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
        <Flex direction={FlexDirection.COLUMN} gap={LINE_GAP} padding={[8, 12]}>
          <ActivityList style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((item) => (
              <ActivityListLine
                key={item.key}
                ref={virtualizer.measureElement}
                data-index={item.index}
                style={{ transform: `translateY(${item.start}px)` }}
              >
                <PipelineCanvasPanelActivityLine event={events[item.index]} />
              </ActivityListLine>
            ))}
          </ActivityList>
          {isStreaming && (
            <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} family={FontFamily.MONO}>
              <Flasher isFlashing>Listening...</Flasher>
            </Text>
          )}
        </Flex>
      </ScrollArea>
    </FlexItem>
  );
};

export default PipelineCanvasPanelActivity;
