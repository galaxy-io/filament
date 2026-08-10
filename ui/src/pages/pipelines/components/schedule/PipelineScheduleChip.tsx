import { create } from "@bufbuild/protobuf";
import { CalendarIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { GetPipelineRequestSchema, type PipelineSchedule } from "@/gen/ingestion/v1/pipelines_pb";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

import { formatTimeUntil } from "@/utils/format";

interface PipelineScheduleChipProps {
  pipelineId: string;
  schedule?: PipelineSchedule;
}

const PipelineScheduleChip = ({
  pipelineId,
  schedule: initialSchedule,
}: PipelineScheduleChipProps) => {
  const { data, isLoading } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id: pipelineId }),
    options: { enabled: !initialSchedule },
  });
  const schedule = data?.schedule ?? initialSchedule;

  if (isLoading) {
    return (
      <Chip
        icon={CalendarIcon}
        label="Next run"
        variant={ChipVariant.YELLOW}
        size={ChipSize.SMALL}
        isPill
        isLoading
      />
    );
  }

  if (!schedule?.config?.enabled || schedule.nextFireAt === 0n) {
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
