import type { FC, ReactNode } from "react";

import Alert from "@galaxy-io/dls/feedback/Alert";

import {
  WEBHOOK_ENDPOINT_STATUS_TO_HUE_MAP,
  WEBHOOK_ENDPOINT_STATUS_TO_MESSAGE_MAP,
} from "@/components/webhooks/constants";
import type { WebhookEndpointStatus } from "@/components/webhooks/types";

interface WebhookEndpointStatusCalloutProps {
  status: WebhookEndpointStatus;
  actions?: ReactNode;
}

const WebhookEndpointStatusCallout: FC<WebhookEndpointStatusCalloutProps> = ({
  status,
  actions,
}) => (
  <Alert
    color={WEBHOOK_ENDPOINT_STATUS_TO_HUE_MAP[status]}
    header={WEBHOOK_ENDPOINT_STATUS_TO_MESSAGE_MAP[status]}
    actions={actions}
  />
);

export default WebhookEndpointStatusCallout;
