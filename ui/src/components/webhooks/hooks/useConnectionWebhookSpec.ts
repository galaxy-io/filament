import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { isWebhookReceiving } from "@/components/webhooks/utils";

import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";

export const useConnectionWebhookSpec = (connection: Connection | null | undefined) => {
  const { data, isLoading } = useGetConnectorQuery({
    input: createGetConnectorInput(connection?.connector ?? "", connection?.kind),
    options: { enabled: !!connection },
  });
  const webhook = data?.connector?.webhook;
  return {
    webhook: isWebhookReceiving(webhook, connection?.config) ? webhook : undefined,
    isLoading,
  };
};
