import {
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
} from "@connectrpc/connect-query";

import type {
  DiscoverResourcesRequest,
  DiscoverResourcesResponse,
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
  >(IngestionService.method.listConnectors, input, options);
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
