import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
} from "@connectrpc/connect-query";

import type {
  DiscoverResourcesRequest,
  DiscoverResourcesResponse,
  GetConnectorRequest,
  GetConnectorResponse,
  GetResourceColumnsRequest,
  GetResourceColumnsResponse,
  ListConnectorsRequest,
  ListConnectorsResponse,
} from "@/gen/ingestion/v1/providers_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

export const useListConnectorsQuery = ({
  input,
  options = {},
}: {
  input?: ListConnectorsRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.listConnectors.output,
    ListConnectorsResponse
  >;
} = {}) => {
  return useQuery<
    typeof IngestionService.method.listConnectors.input,
    typeof IngestionService.method.listConnectors.output
  >(IngestionService.method.listConnectors, input, { staleTime: Infinity, ...options });
};

export const useGetConnectorQuery = ({
  input,
  options = {},
}: {
  input: GetConnectorRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getConnector.output,
    GetConnectorResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getConnector.input,
    typeof IngestionService.method.getConnector.output
  >(IngestionService.method.getConnector, input, { staleTime: Infinity, ...options });
};

export const createDiscoverResourcesQueryKey = (
  input?: DiscoverResourcesRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.discoverResources,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useDiscoverResourcesQuery = ({
  input,
  options = {},
}: {
  input: DiscoverResourcesRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.discoverResources.output,
    DiscoverResourcesResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.discoverResources.input,
    typeof IngestionService.method.discoverResources.output
  >(IngestionService.method.discoverResources, input, options);
};

export const useGetResourceColumnsQuery = ({
  input,
  options = {},
}: {
  input: GetResourceColumnsRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getResourceColumns.output,
    GetResourceColumnsResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getResourceColumns.input,
    typeof IngestionService.method.getResourceColumns.output
  >(IngestionService.method.getResourceColumns, input, options);
};

export const useValidateConfigMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.validateConfig.input,
    typeof IngestionService.method.validateConfig.output
  > = {},
) => {
  return useMutation<
    typeof IngestionService.method.validateConfig.input,
    typeof IngestionService.method.validateConfig.output
  >(IngestionService.method.validateConfig, options);
};
