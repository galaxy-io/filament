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
import { useIsFetching } from "@tanstack/react-query";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type DiscoverResourcesRequest,
  DiscoverResourcesRequestSchema,
  type DiscoverResourcesResponse,
  type GetConnectorRequest,
  type GetConnectorResponse,
  type GetResourceColumnsRequest,
  type GetResourceColumnsResponse,
  type ListConnectorsRequest,
  type ListConnectorsResponse,
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

export const useIsDiscoveringResources = (connectionId: Connection["id"] = "") => {
  const transport = useTransport();
  return (
    useIsFetching({
      queryKey: createDiscoverResourcesQueryKey(
        create(DiscoverResourcesRequestSchema, { connectionId }),
        transport,
      ),
    }) > 0
  );
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
