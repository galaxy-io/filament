import { useCallback } from "react";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";
import type { WebhookEndpointConfig } from "@/gen/ingestion/v1/webhooks_pb";

import { createPutWebhookEndpointRequest } from "@/components/webhooks/utils";

import type { ConnectionFormSigningKeyReveal } from "@/pages/connections/components/form/types";

import { usePutWebhookEndpointMutation } from "@/api/queries/webhooks";

interface ConnectionWebhookKeyMintCallbacks {
  onSuccess: (reveal: ConnectionFormSigningKeyReveal) => void;
  onError: (error: unknown) => void;
}

export const useConnectionWebhookKeyMint = () => {
  const { mutate: putEndpoint, reset, isPending } = usePutWebhookEndpointMutation();

  const mintKey = useCallback(
    (
      connection: Connection,
      webhook: WebhookSpec,
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
              webhook,
              signingKey: response.mintedSecret,
              deliveryUrl: response.endpoint?.deliveryUrl ?? "",
            });
          },
          onError,
        },
      ),
    [putEndpoint, reset],
  );

  return { mintKey, isPending };
};
