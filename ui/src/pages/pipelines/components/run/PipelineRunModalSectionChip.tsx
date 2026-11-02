import type { FC } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

interface PipelineRunModalSectionChipProps {
  isDefault: boolean;
}

const PipelineRunModalSectionChip: FC<PipelineRunModalSectionChipProps> = ({ isDefault }) =>
  isDefault ? (
    <Chip label="Default" size={ChipSize.SMALL} variant={ChipVariant.SECONDARY} />
  ) : (
    <Chip label="Custom" size={ChipSize.SMALL} variant={ChipVariant.WARNING} />
  );

export default PipelineRunModalSectionChip;
