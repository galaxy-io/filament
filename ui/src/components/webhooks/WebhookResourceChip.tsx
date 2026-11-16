import type { FC } from "react";

import { WebhooksLogoIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";

const WebhookResourceChip: FC = () => (
  <Chip
    icon={WebhooksLogoIcon}
    label="Webhook"
    size={ChipSize.SMALL}
    tooltip="Reads webhook deliveries continuously."
  />
);

export default WebhookResourceChip;
