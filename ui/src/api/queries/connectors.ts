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
} from "@/gen/ingestion/v1/connectors_pb";
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
  return useQuery(IngestionService.method.listConnectors, input, {
    staleTime: Number.POSITIVE_INFINITY,
    ...options,
  });
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
  return useQuery(IngestionService.method.getConnector, input, {
    staleTime: Number.POSITIVE_INFINITY,
    ...options,
  });
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
  return useQuery(IngestionService.method.discoverResources, input, options);
};

export const createGetResourceColumnsQueryKey = (
  input?: GetResourceColumnsRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getResourceColumns,
    input,
    transport,
    cardinality: "finite",
  });
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
  return useQuery(IngestionService.method.getResourceColumns, input, options);
};

export const useValidateConfigMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.validateConfig.input,
    typeof IngestionService.method.validateConfig.output
  > = {},
) => {
  return useMutation(IngestionService.method.validateConfig, options);
};
