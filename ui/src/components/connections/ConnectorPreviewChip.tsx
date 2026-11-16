import type { FC } from "react";

import { FlaskIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

interface ConnectorPreviewChipProps {
  isPreview: ConnectorSpec["isPreview"];
}

const ConnectorPreviewChip: FC<ConnectorPreviewChipProps> = ({ isPreview }) =>
  isPreview ? (
    <Chip
      icon={FlaskIcon}
      ariaLabel="This connector is in preview"
      tooltip="This connector is in preview"
      size={ChipSize.SMALL}
    />
  ) : null;

export default ConnectorPreviewChip;
