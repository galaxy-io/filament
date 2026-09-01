import { useRef, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { PulseIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";
import { useVirtualizer } from "@tanstack/react-virtual";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Flash from "@galaxy-io/dls/transform/Flash";

import { ListRunsRequestSchema, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import { PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS } from "@/pages/pipelines/canvas/panel/activity/constants";
import PipelineCanvasPanelActivityLine from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivityLine";
import { getRunEventKey } from "@/pages/pipelines/canvas/panel/activity/utils";

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useListRunsQuery, useTailRunsStream } from "@/api/queries/runs";

const ESTIMATED_LINE_HEIGHT = 18;
const LINE_GAP = 2;

const ActivityBody = styled.div`
  flex: 1;
  min-height: 0;
  padding: 8px 12px;

  display: flex;
  flex-direction: column;
  gap: ${LINE_GAP}px;

  overflow-y: auto;
`;

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

const PipelineCanvasPanelActivity = () => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data: activeRunsData } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
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
      <ActivityBody>
        <EmptyLayout
          icon={<Icon component={PulseIcon} size={16} variant={IconVariant.SECONDARY} />}
          message="Run the pipeline to see activity"
        />
      </ActivityBody>
    );
  }

  return (
    <ActivityBody ref={bodyRef}>
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
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
          <Flash>Listening...</Flash>
        </Text>
      )}
    </ActivityBody>
  );
};

export default PipelineCanvasPanelActivity;
