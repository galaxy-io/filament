import { useCallback } from "react";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { WebhookEndpointConfig } from "@/gen/ingestion/v1/webhooks_pb";

import {
  createPutWebhookEndpointRequest,
  createWebhookDeliveryBaseUrl,
} from "@/components/webhooks/utils";

import type { ConnectionFormSigningKeyReveal } from "@/pages/connections/components/form/types";

import { usePutWebhookEndpointMutation } from "@/api/queries/webhooks";

interface ConnectionWebhookKeyMintCallbacks {
  onSuccess: (reveal: ConnectionFormSigningKeyReveal) => void;
  onError: (error: unknown) => void;
}

interface ConnectionWebhookKeyImportCallbacks {
  onSuccess: () => void;
  onError: (error: unknown) => void;
}

export const useConnectionWebhookKeyMint = () => {
  const { mutate: putEndpoint, reset, isPending } = usePutWebhookEndpointMutation();

  const mintKey = useCallback(
    (
      connection: Connection,
      endpoint: WebhookEndpointConfig | undefined,
      { onSuccess, onError }: ConnectionWebhookKeyMintCallbacks,
    ) =>
      putEndpoint(
        createPutWebhookEndpointRequest({
          sourceConnectionId: connection.id,
          endpoint,
          mintSecret: true,
        }),
        {
          onSuccess: (response) => {
            reset();
            onSuccess({
              connection,
              signingKey: response.mintedSecret,
              deliveryUrl: createWebhookDeliveryBaseUrl(response.endpoint?.deliveryUrl),
            });
          },
          onError,
        },
      ),
    [putEndpoint, reset],
  );

  const importKey = useCallback(
    (
      connection: Connection,
      endpoint: WebhookEndpointConfig | undefined,
      importedSecret: string,
      { onSuccess, onError }: ConnectionWebhookKeyImportCallbacks,
    ) =>
      putEndpoint(
        createPutWebhookEndpointRequest({
          sourceConnectionId: connection.id,
          endpoint,
          importedSecret,
        }),
        {
          onSuccess: () => {
            reset();
            onSuccess();
          },
          onError,
        },
      ),
    [putEndpoint, reset],
  );

  return { mintKey, importKey, isPending };
};
