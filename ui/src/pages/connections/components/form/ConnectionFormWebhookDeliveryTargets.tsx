import type { FC } from "react";

import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";

import {
  WEBHOOK_DELIVERY_URL_UNAVAILABLE_MESSAGE,
  WEBHOOK_DYNAMIC_DELIVERY_URL_TOOLTIP,
} from "@/components/webhooks/constants";
import { createWebhookDeliveryUrl, getWebhookDeliveryTargets } from "@/components/webhooks/utils";

interface WebhookDeliveryTargetsProps {
  webhook: WebhookSpec;
  deliveryUrl: string;
}

const ConnectionFormWebhookDeliveryTargets: FC<WebhookDeliveryTargetsProps> = ({
  webhook,
  deliveryUrl,
}) => {
  if (!deliveryUrl) {
    return <Alert variant={AlertVariant.WARNING}>{WEBHOOK_DELIVERY_URL_UNAVAILABLE_MESSAGE}</Alert>;
  }

  return (
    <>
      {getWebhookDeliveryTargets(webhook).map((target) => (
        <CopyInput
          key={target.label}
          label={target.label}
          labelTooltip={webhook.dynamicResources ? WEBHOOK_DYNAMIC_DELIVERY_URL_TOOLTIP : undefined}
          value={createWebhookDeliveryUrl(target.resource, deliveryUrl) ?? ""}
          family={FontFamily.MONO}
          fillWidth
        />
      ))}
    </>
  );
};

export default ConnectionFormWebhookDeliveryTargets;
