import { type FC, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";
import type { WebhookEndpointConfig } from "@/gen/ingestion/v1/webhooks_pb";

import {
  createPutWebhookEndpointRequest,
  getWebhookConnectorTileBadge,
  getWebhookSecretError,
} from "@/components/webhooks/utils";

import ConnectionFormHeader from "@/pages/connections/components/form/ConnectionFormHeader";
import ConnectionFormWebhookDeliveryTargets from "@/pages/connections/components/form/ConnectionFormWebhookDeliveryTargets";
import ConnectionFormWebhookSetupGuide from "@/pages/connections/components/form/ConnectionFormWebhookSetupGuide";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";

import {
  createGetWebhookDeliveryURLInput,
  useGetWebhookDeliveryURLQuery,
  usePutWebhookEndpointMutation,
} from "@/api/queries/webhooks";

import { getErrorMessage } from "@/utils/errors";

interface ConnectionFormWebhookSecretModalProps {
  isOpen: boolean;
  connection: Connection;
  webhook: WebhookSpec;
  endpoint: WebhookEndpointConfig | undefined;
  isNewConnection: boolean;
  onDone: () => void;
}

const ConnectionFormWebhookSecretModal: FC<ConnectionFormWebhookSecretModalProps> = ({
  isOpen,
  connection,
  webhook,
  endpoint,
  isNewConnection,
  onDone,
}) => {
  const { toast } = useToast();
  const [secret, setSecret] = useState("");
  const trimmedSecret = secret.trim();
  const secretError = getWebhookSecretError(trimmedSecret);

  const {
    data,
    error: deliveryUrlError,
    isError,
    isLoading,
  } = useGetWebhookDeliveryURLQuery({
    input: createGetWebhookDeliveryURLInput(connection.id),
  });
  const deliveryUrl = data?.deliveryUrl ?? "";

  const { mutate: putEndpoint, isPending } = usePutWebhookEndpointMutation();

  const handleSave = () => {
    putEndpoint(
      createPutWebhookEndpointRequest({
        sourceConnectionId: connection.id,
        endpoint,
        importedSecret: trimmedSecret,
      }),
      {
        onSuccess: () => {
          toast({ variant: ToastVariant.SUCCESS, header: "Signing key saved" });
          onDone();
        },
        onError: (error) => {
          toast({
            variant: ToastVariant.ERROR,
            header: "Save failed",
            description: getErrorMessage(error, "Could not save the signing key"),
          });
        },
      },
    );
  };

  const renderDeliveryUrl = () => {
    if (isLoading) {
      return (
        <Box fillWidth>
          <Skeleton lines={2} />
        </Box>
      );
    }

    if (isError) {
      return (
        <Alert variant={AlertVariant.ERROR} header="Delivery URL not loaded">
          {getErrorMessage(deliveryUrlError, "Could not load the delivery URL")}
        </Alert>
      );
    }

    return (
      <>
        <ConnectionFormWebhookDeliveryTargets webhook={webhook} deliveryUrl={deliveryUrl} />
        <ConnectionFormWebhookSetupGuide webhook={webhook} deliveryUrl={deliveryUrl} />
      </>
    );
  };

  return (
    <ConnectionFormWrapper
      isOpen={isOpen}
      header={
        <ConnectionFormHeader
          connectorName={connection.connector}
          connectorKind={connection.kind}
          title={`${connection.name} signing key`}
          badge={getWebhookConnectorTileBadge(webhook)}
        />
      }
      footer={
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button
            label={isNewConnection ? "Set up later" : "Cancel"}
            variant={ButtonVariant.SECONDARY}
            isDisabled={isPending}
            onClick={onDone}
          />
          <Button
            label="Save key"
            variant={ButtonVariant.PRIMARY}
            hotKeys={["mod", "enter"]}
            shouldBindHotKey
            isLoading={isPending}
            isDisabled={trimmedSecret === "" || !!secretError}
            onClick={handleSave}
          />
        </Flex>
      }
      onClose={onDone}
    >
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
        {renderDeliveryUrl()}
        <PasswordInput
          label="Signing key"
          labelTooltip="The key the provider shows for this webhook. Filament verifies every delivery with it."
          value={secret}
          error={secretError}
          onChange={setSecret}
          placeholder="Paste the signing key..."
          isDisabled={isPending}
          isRequired
          fillWidth
        />
      </Flex>
    </ConnectionFormWrapper>
  );
};

export default ConnectionFormWebhookSecretModal;
