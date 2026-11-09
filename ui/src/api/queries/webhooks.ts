import { create } from "@bufbuild/protobuf";
import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";
import {
  type GetWebhookEndpointRequest,
  GetWebhookEndpointRequestSchema,
  type GetWebhookEndpointResponse,
} from "@/gen/ingestion/v1/webhooks_pb";

import { createDiscoverResourcesQueryKey } from "@/api/queries/connectors";

export const createGetWebhookEndpointInput = (sourceConnectionId: Connection["id"]) =>
  create(GetWebhookEndpointRequestSchema, { sourceConnectionId });

const createGetWebhookEndpointQueryKey = (
  input?: GetWebhookEndpointRequest,
  transport?: Transport,
) =>
  createConnectQueryKey({
    schema: IngestionService.method.getWebhookEndpoint,
    input,
    transport,
    cardinality: "finite",
  });

export const useGetWebhookEndpointQuery = ({
  input,
  options = {},
}: {
  input: GetWebhookEndpointRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getWebhookEndpoint.output,
    GetWebhookEndpointResponse
  >;
}) => useQuery(IngestionService.method.getWebhookEndpoint, input, options);

export const usePutWebhookEndpointMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.putWebhookEndpoint.input,
    typeof IngestionService.method.putWebhookEndpoint.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.putWebhookEndpoint, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createGetWebhookEndpointQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createDiscoverResourcesQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};
