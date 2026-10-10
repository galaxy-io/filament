import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { CalendarIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import { formatRelativeTime } from "@galaxy-io/dls/utils/format";

import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

interface PipelineScheduleChipProps {
  pipelineId: Pipeline["id"];
}

const PipelineScheduleChip: FC<PipelineScheduleChipProps> = ({ pipelineId }) => {
  const { data, isLoading } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id: pipelineId, includeSchedule: true }),
    options: { enabled: Boolean(pipelineId) },
  });
  const schedule = data?.pipeline?.schedule;

  if (isLoading) {
    return null;
  }

  if (!schedule?.config?.isEnabled || !schedule?.nextFireAt || schedule.nextFireAt === 0n) {
    return null;
  }

  return (
    <Chip
      icon={CalendarIcon}
      label={`Next run ${formatRelativeTime(schedule.nextFireAt)}`}
      color="yellow"
      size={ChipSize.SMALL}
      isPill
    />
  );
};

export default PipelineScheduleChip;
