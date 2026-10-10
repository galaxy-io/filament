import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { CalendarIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";

import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

import { formatTimeUntil } from "@/utils/format";

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
      label={`Next run ${formatTimeUntil(schedule.nextFireAt)}`}
      color="yellow"
      size={ChipSize.SMALL}
      isPill
    />
  );
};

export default PipelineScheduleChip;
