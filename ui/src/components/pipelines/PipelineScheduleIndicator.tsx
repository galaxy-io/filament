import type { FC } from "react";

import { CalendarIcon } from "@phosphor-icons/react";

import Icon from "@galaxy-io/dls/icons/Icon";
import { formatRelativeTime } from "@galaxy-io/dls/utils/format";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_SCHEDULE_INDICATOR_SIZE } from "@/components/pipelines/constants";
import { getPipelineNextFireAt } from "@/components/pipelines/utils";

import { createGetPipelineInput, useGetPipelineQuery } from "@/api/queries/pipelines";

interface PipelineScheduleIndicatorProps {
  pipelineId: Pipeline["id"];
  pipeline?: Pipeline;
}

const PipelineScheduleIndicator: FC<PipelineScheduleIndicatorProps> = ({
  pipelineId,
  pipeline,
}) => {
  const { data } = useGetPipelineQuery({
    input: createGetPipelineInput(pipelineId),
    options: { enabled: !pipeline },
  });
  const nextFireAt = getPipelineNextFireAt(pipeline ?? data?.pipeline);

  if (!nextFireAt) {
    return null;
  }

  return (
    <Icon
      component={CalendarIcon}
      size={PIPELINE_SCHEDULE_INDICATOR_SIZE}
      color="yellow"
      ariaLabel="Scheduled"
      tooltip={`Next run ${formatRelativeTime(nextFireAt)}`}
    />
  );
};

export default PipelineScheduleIndicator;
