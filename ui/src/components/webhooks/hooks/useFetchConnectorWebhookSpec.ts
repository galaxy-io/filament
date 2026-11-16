import { useCallback } from "react";

import { useTransport } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { createGetConnectorInput, createGetConnectorQueryOptions } from "@/api/queries/connectors";

import { ignoreNotFoundError } from "@/utils/errors";

export const useFetchConnectorWebhookSpec = () => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  return useCallback(
    (connector: Connection["connector"], kind: ConnectorKind) =>
      queryClient
        .fetchQuery(
          createGetConnectorQueryOptions({
            input: createGetConnectorInput(connector, kind),
            transport,
          }),
        )
        .then((response) => response.connector?.webhook, ignoreNotFoundError),
    [queryClient, transport],
  );
};
