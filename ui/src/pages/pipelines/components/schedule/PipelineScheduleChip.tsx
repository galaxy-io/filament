import { create } from "@bufbuild/protobuf";
import { CalendarIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

import { formatTimeUntil } from "@/utils/format";

interface PipelineScheduleChipProps {
  pipelineId: Pipeline["id"];
}

const PipelineScheduleChip = ({ pipelineId }: PipelineScheduleChipProps) => {
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
      variant={ChipVariant.YELLOW}
      size={ChipSize.SMALL}
      isPill
    />
  );
};

export default PipelineScheduleChip;
