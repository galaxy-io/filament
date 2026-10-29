import type { FC } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

interface PipelineResourceColumnsChipProps {
  selectedCount: number;
  totalCount: number;
}

const PipelineResourceColumnsChip: FC<PipelineResourceColumnsChipProps> = ({
  selectedCount,
  totalCount,
}) => (
  <Chip
    label={`${selectedCount} of ${totalCount}`}
    tooltip="Selected columns"
    size={ChipSize.SMALL}
    variant={selectedCount === totalCount ? ChipVariant.SECONDARY : ChipVariant.PRIMARY}
  />
);

export default PipelineResourceColumnsChip;
