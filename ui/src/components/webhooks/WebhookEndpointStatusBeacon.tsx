import type { FC } from "react";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";

import {
  WEBHOOK_ENDPOINT_STATUS_TO_HUE_MAP,
  WEBHOOK_ENDPOINT_STATUS_TO_LABEL_MAP,
} from "@/components/webhooks/constants";
import { WebhookEndpointStatus } from "@/components/webhooks/types";

interface WebhookEndpointStatusBeaconProps {
  status: WebhookEndpointStatus;
}

const WebhookEndpointStatusBeacon: FC<WebhookEndpointStatusBeaconProps> = ({ status }) => {
  const hue = WEBHOOK_ENDPOINT_STATUS_TO_HUE_MAP[status];
  const label = WEBHOOK_ENDPOINT_STATUS_TO_LABEL_MAP[status];

  if (!hue) {
    return <Beacon variant={BeaconVariant.TERTIARY} label={label} />;
  }

  return <Beacon color={hue} label={label} isPulse={status === WebhookEndpointStatus.LIVE} />;
};

export default WebhookEndpointStatusBeacon;
