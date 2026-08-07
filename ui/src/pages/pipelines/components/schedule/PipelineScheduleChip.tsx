import { CalendarIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

import type { PipelineSchedule } from "@/gen/ingestion/v1/pipelines_pb";

import { formatTimeUntil } from "@/utils/format";

interface PipelineScheduleChipProps {
  schedule?: PipelineSchedule;
}

const PipelineScheduleChip = ({ schedule }: PipelineScheduleChipProps) => {
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
