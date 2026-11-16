import { type FC, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import MarkdownText from "@galaxy-io/dls/text/MarkdownText";
import Text from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { formatConnectorName } from "@/components/connections/utils";
import { WEBHOOK_SIGNING_SECRET_MIN_LENGTH } from "@/components/webhooks/constants";
import { createWebhookSetupContent } from "@/components/webhooks/utils";

import ConnectionFormHeader from "@/pages/connections/components/form/ConnectionFormHeader";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import { useConnectionWebhookKeyMint } from "@/pages/connections/hooks/useConnectionWebhookKeyMint";

import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";
import {
  createGetWebhookDeliveryURLInput,
  useGetWebhookDeliveryURLQuery,
} from "@/api/queries/webhooks";

import { getErrorMessage } from "@/utils/errors";

interface ConnectionFormWebhookSecretModalProps {
  isOpen: boolean;
  connection: Connection;
  onDone: () => void;
}

interface ConnectionFormWebhookSecretModalState {
  secret: string;
  shouldShowError: boolean;
}

const DEFAULT_STATE: ConnectionFormWebhookSecretModalState = {
  secret: "",
  shouldShowError: false,
};

const ConnectionFormWebhookSecretModal: FC<ConnectionFormWebhookSecretModalProps> = ({
  isOpen,
  connection,
  onDone,
}) => {
  const { toast } = useToast();
  const [state, setState] = useState<ConnectionFormWebhookSecretModalState>(DEFAULT_STATE);
  const { importKey, isPending } = useConnectionWebhookKeyMint();
  const { data: connectorData } = useGetConnectorQuery({
    input: createGetConnectorInput(connection.connector, connection.kind),
  });
  const { data, isLoading } = useGetWebhookDeliveryURLQuery({
    input: createGetWebhookDeliveryURLInput(connection.id),
    options: { retry: false },
  });

  const connector = connectorData?.connector;
  const providerName = connector ? formatConnectorName(connector) : "the provider";
  const setup = connector?.webhook?.setup ?? "";
  const deliveryUrl = data?.deliveryUrl || undefined;
  const isValid = state.secret.length >= WEBHOOK_SIGNING_SECRET_MIN_LENGTH;
  const error =
    state.shouldShowError && !isValid
      ? `Use at least ${WEBHOOK_SIGNING_SECRET_MIN_LENGTH} characters`
      : undefined;

  const handleSecretChange = (secret: string) => {
    setState({ secret, shouldShowError: false });
  };

  const handleSave = () => {
    if (!isValid) {
      setState((prev) => ({ ...prev, shouldShowError: true }));
      return;
    }
    importKey(connection, undefined, state.secret, {
      onSuccess: onDone,
      onError: (error) => {
        toast({
          variant: ToastVariant.ERROR,
          header: "Signing secret not saved",
          description: getErrorMessage(error, "Failed to save the signing secret"),
        });
      },
    });
  };

  const renderDeliveryUrl = () => {
    if (isLoading) {
      return (
        <Box height={56} fillWidth>
          <Skeleton lines={2} />
        </Box>
      );
    }
    if (!deliveryUrl) {
      return <Text>Delivery URL unavailable. Configure WEBHOOK_PUBLIC_BASE_URL.</Text>;
    }
    return (
      <CopyInput
        label="Delivery URL"
        labelTooltip={`Add this URL as a webhook in ${providerName}.`}
        value={deliveryUrl}
        family={FontFamily.MONO}
        fillWidth
      />
    );
  };

  return (
    <ConnectionFormWrapper
      isOpen={isOpen}
      header={
        <ConnectionFormHeader
          connectorName={connection.connector}
          connectorKind={connection.kind}
          title={`${connection.name} webhook`}
        />
      }
      footer={
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button label="Set up later" variant={ButtonVariant.SECONDARY} onClick={onDone} />
          <Button label="Save secret" isLoading={isPending} onClick={handleSave} />
        </Flex>
      }
      onClose={onDone}
    >
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
        {setup && <MarkdownText content={createWebhookSetupContent(setup, deliveryUrl)} />}
        {renderDeliveryUrl()}
        <PasswordInput
          label="Signing secret"
          labelTooltip={`Filament verifies every delivery with the secret ${providerName} creates for this webhook.`}
          value={state.secret}
          onChange={handleSecretChange}
          placeholder="Paste the secret..."
          error={error}
          isRequired
          isDisabled={isPending}
          fillWidth
        />
      </Flex>
    </ConnectionFormWrapper>
  );
};

export default ConnectionFormWebhookSecretModal;
