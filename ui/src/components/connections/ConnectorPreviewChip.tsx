import type { FC } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

interface ConnectorPreviewChipProps {
  isPreview: ConnectorSpec["isPreview"];
}

const ConnectorPreviewChip: FC<ConnectorPreviewChipProps> = ({ isPreview }) =>
  isPreview ? (
    <Chip
      label="Preview"
      tooltip="This connector is in preview"
      variant={ChipVariant.TERTIARY}
      size={ChipSize.SMALL}
    />
  ) : null;

export default ConnectorPreviewChip;
