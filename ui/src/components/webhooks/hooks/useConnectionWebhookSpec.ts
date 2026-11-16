import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { isWebhookReceiving } from "@/components/webhooks/utils";

import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";

export const useConnectionWebhookSpec = (
  connection: Pick<Connection, "connector" | "kind" | "config"> | null | undefined,
) => {
  const isSource = connection?.kind === ConnectorKind.SOURCE;
  const { data, isLoading } = useGetConnectorQuery({
    input: createGetConnectorInput(connection?.connector ?? "", ConnectorKind.SOURCE),
    options: { enabled: isSource && !!connection?.connector },
  });
  const spec = data?.connector?.webhook;
  return {
    webhookSpec: isSource && isWebhookReceiving(spec, connection?.config) ? spec : undefined,
    isLoading,
  };
};
