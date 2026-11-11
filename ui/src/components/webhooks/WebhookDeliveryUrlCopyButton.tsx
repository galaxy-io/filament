import type { FC } from "react";

import { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import CopyButton from "@galaxy-io/dls/buttons/CopyButton";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { createWebhookDeliveryUrl } from "@/components/webhooks/utils";

import { createGetWebhookEndpointInput, useGetWebhookEndpointQuery } from "@/api/queries/webhooks";

interface WebhookDeliveryUrlCopyButtonProps {
  sourceConnectionId: Connection["id"];
  resource: string;
}

const WebhookDeliveryUrlCopyButton: FC<WebhookDeliveryUrlCopyButtonProps> = ({
  sourceConnectionId,
  resource,
}) => {
  const { data, isLoading } = useGetWebhookEndpointQuery({
    input: createGetWebhookEndpointInput(sourceConnectionId),
    options: { retry: false },
  });
  const deliveryUrl = createWebhookDeliveryUrl(
    sourceConnectionId,
    resource,
    data?.endpoint?.deliveryUrl,
  );

  return (
    <Tooltip
      body={
        <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
          {deliveryUrl}
        </Text>
      }
    >
      <CopyButton
        label="Copy URL"
        value={deliveryUrl}
        ariaLabel={`Copy ${resource} delivery URL`}
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        isDisabled={isLoading}
      />
    </Tooltip>
  );
};

export default WebhookDeliveryUrlCopyButton;
