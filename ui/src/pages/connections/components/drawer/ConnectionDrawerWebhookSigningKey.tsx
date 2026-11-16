import { type FC, useState } from "react";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { WebhookEndpointConfig } from "@/gen/ingestion/v1/webhooks_pb";

import { PIPELINE_CARD_HEIGHT } from "@/components/pipelines/constants";
import { isWebhookPreviousKeyActive } from "@/components/webhooks/utils";

import ConnectionFormSigningKeyModal from "@/pages/connections/components/form/ConnectionFormSigningKeyModal";
import type { ConnectionFormSigningKeyReveal } from "@/pages/connections/components/form/types";
import { useConnectionWebhookKeyMint } from "@/pages/connections/hooks/useConnectionWebhookKeyMint";

import { type UseConfirmCallbacks, useConfirm } from "@/hooks/useConfirm";

import { getErrorMessage } from "@/utils/errors";
import { formatTimestamp } from "@/utils/format";

interface ConnectionDrawerWebhookSigningKeyProps {
  connection: Connection;
  endpoint: WebhookEndpointConfig | undefined;
}

const ConnectionDrawerWebhookSigningKey: FC<ConnectionDrawerWebhookSigningKeyProps> = ({
  connection,
  endpoint,
}) => {
  const { toast } = useToast();
  const [reveal, setReveal] = useState<ConnectionFormSigningKeyReveal | undefined>();
  const { mintKey, isPending } = useConnectionWebhookKeyMint();

  const hasKey = !!endpoint?.hasSigningSecret;
  const isPreviousKeyActive = isWebhookPreviousKeyActive(endpoint);
  const previousKeyExpiry = formatTimestamp(endpoint?.previousKeyExpiresAt ?? 0n);

  const mintSigningKey = (callbacks: UseConfirmCallbacks) =>
    mintKey(connection, endpoint, {
      onSuccess: (reveal) => {
        setReveal(reveal);
        callbacks.onSuccess();
      },
      onError: callbacks.onError,
    });

  const rotateConfirm = useConfirm<Connection>({
    entityLabel: "Signing key",
    messages: {
      successHeader: "Signing key rotated",
      errorHeader: "Rotate failed",
      errorFallback: "Could not rotate the signing key",
    },
    onConfirm: (_, callbacks) => mintSigningKey(callbacks),
  });

  const handleGenerate = () =>
    mintSigningKey({
      onSuccess: () => undefined,
      onError: (error) =>
        toast({
          variant: ToastVariant.ERROR,
          header: "Generate failed",
          description: getErrorMessage(error, "Could not generate a signing key"),
        }),
    });

  return (
    <>
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} fillWidth>
        <Flex
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          gap={12}
          height={PIPELINE_CARD_HEIGHT}
          padding={[0, 12]}
          fillWidth
        >
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
            Signing key
          </Text>
          <Flex alignItems={AlignItems.CENTER} gap={12}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              {hasKey ? "HMAC-SHA256" : "Not set"}
            </Text>
            {hasKey ? (
              <Button
                label="Rotate"
                size={ButtonSize.SMALL}
                isDisabled={isPreviousKeyActive}
                tooltip={
                  isPreviousKeyActive ? `Rotate again after ${previousKeyExpiry}` : undefined
                }
                onClick={() => rotateConfirm.handleOpen(connection)}
              />
            ) : (
              <Button
                label="Generate"
                size={ButtonSize.SMALL}
                isLoading={isPending}
                onClick={handleGenerate}
              />
            )}
          </Flex>
        </Flex>
        {isPreviousKeyActive && (
          <>
            <Divider />
            <Flex
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              gap={12}
              height={PIPELINE_CARD_HEIGHT}
              padding={[0, 12]}
              fillWidth
            >
              <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
                Previous key
              </Text>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Works until {previousKeyExpiry}
              </Text>
            </Flex>
          </>
        )}
      </Flex>
      <ConfirmDialog
        isOpen={rotateConfirm.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) rotateConfirm.handleClose();
        }}
        onConfirm={rotateConfirm.handleConfirm}
        header="Rotate signing key?"
        description="Filament creates a new key and shows it once. The current key keeps working for 72 hours."
        label="Rotate key"
      />
      {reveal && (
        <ConnectionFormSigningKeyModal isOpen reveal={reveal} onDone={() => setReveal(undefined)} />
      )}
    </>
  );
};

export default ConnectionDrawerWebhookSigningKey;
