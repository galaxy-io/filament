import type { FC } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

interface PipelineResourceColumnsChipProps {
  selectedCount: number;
  totalCount: number;
  isRaised?: boolean;
}

const PipelineResourceColumnsChip: FC<PipelineResourceColumnsChipProps> = ({
  selectedCount,
  totalCount,
  isRaised = false,
}) => {
  const fullVariant = isRaised ? ChipVariant.TERTIARY : ChipVariant.SECONDARY;

  return (
    <Chip
      label={`${selectedCount} of ${totalCount}`}
      tooltip="Selected columns"
      size={ChipSize.SMALL}
      variant={selectedCount === totalCount ? fullVariant : ChipVariant.PRIMARY}
    />
  );
};

export default PipelineResourceColumnsChip;
