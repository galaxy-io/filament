import type { FC } from "react";

import { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import CopyButton from "@galaxy-io/dls/buttons/CopyButton";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { WEBHOOK_DELIVERY_URL_UNAVAILABLE_MESSAGE } from "@/components/webhooks/constants";
import { createWebhookDeliveryUrl } from "@/components/webhooks/utils";

import {
  createGetWebhookDeliveryURLInput,
  useGetWebhookDeliveryURLQuery,
} from "@/api/queries/webhooks";

import { getErrorMessage } from "@/utils/errors";

interface WebhookDeliveryUrlCopyButtonProps {
  sourceConnectionId: Connection["id"];
  resource: string;
}

const WebhookDeliveryUrlCopyButton: FC<WebhookDeliveryUrlCopyButtonProps> = ({
  sourceConnectionId,
  resource,
}) => {
  const { data, error, isError, isLoading } = useGetWebhookDeliveryURLQuery({
    input: createGetWebhookDeliveryURLInput(sourceConnectionId),
    options: { retry: false },
  });
  const deliveryUrl = createWebhookDeliveryUrl(resource, data?.deliveryUrl);

  return (
    <Tooltip
      body={
        <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
          {isError
            ? getErrorMessage(error, "Could not load the delivery URL")
            : (deliveryUrl ?? WEBHOOK_DELIVERY_URL_UNAVAILABLE_MESSAGE)}
        </Text>
      }
    >
      <CopyButton
        label="Copy URL"
        value={deliveryUrl ?? ""}
        ariaLabel={resource ? `Copy URL for ${resource}` : undefined}
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        isDisabled={isLoading || !deliveryUrl}
      />
    </Tooltip>
  );
};

export default WebhookDeliveryUrlCopyButton;
