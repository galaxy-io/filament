import type { FC } from "react";

import { WebhooksLogoIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";

interface ConnectorWebhooksChipProps {
  hasWebhooks: boolean;
}

const ConnectorWebhooksChip: FC<ConnectorWebhooksChipProps> = ({ hasWebhooks }) =>
  hasWebhooks ? (
    <Chip
      icon={WebhooksLogoIcon}
      label="Webhooks"
      tooltip="This connector can receive webhooks"
      size={ChipSize.SMALL}
    />
  ) : null;

export default ConnectorWebhooksChip;
