import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { PulseIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Flash from "@galaxy-io/dls/transform/Flash";

import { ListRunsRequestSchema, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import { PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS } from "@/pages/pipelines/canvas/panel/activity/constants";
import PipelineCanvasPanelActivityLine from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivityLine";

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useListRunsQuery, useTailRunsStream } from "@/api/queries/runs";

const ActivityBody = styled.div`
  flex: 1;
  min-height: 0;
  padding: 8px 12px;

  display: flex;
  flex-direction: column;
  gap: 2px;

  overflow-y: auto;
`;

const PipelineCanvasPanelActivity = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

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
    <ActivityBody>
      {events.map((event, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: append-only feed without stable identity
        <PipelineCanvasPanelActivityLine key={index} event={event} />
      ))}
      {isStreaming && (
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
          <Flash>Listening...</Flash>
        </Text>
      )}
    </ActivityBody>
  );
};

export default PipelineCanvasPanelActivity;
