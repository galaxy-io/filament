import type { FC } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

interface PipelineResourcePartitionsChipProps {
  selectedCount: number;
  totalCount: number;
}

const PipelineResourcePartitionsChip: FC<PipelineResourcePartitionsChipProps> = ({
  selectedCount,
  totalCount,
}) => (
  <Chip
    label={`${selectedCount} of ${totalCount}`}
    tooltip="Partitions to backfill. Changes stream from every partition."
    size={ChipSize.SMALL}
    variant={selectedCount === totalCount ? ChipVariant.TERTIARY : ChipVariant.PRIMARY}
  />
);

export default PipelineResourcePartitionsChip;
