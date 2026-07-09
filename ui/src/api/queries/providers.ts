import {
  createConnectQueryKey,
  useQuery,
  UseQueryOptions,
} from "@connectrpc/connect-query";

import {
  DiscoverResourcesRequest,
  DiscoverResourcesResponse,
  ListProvidersRequest,
  ListProvidersResponse,
  ValidateConfigRequest,
  ValidateConfigResponse,
} from "@/gen/ingestion/v1/providers_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

const listProviders = IngestionService.method.listProviders;
const validateConfig = IngestionService.method.validateConfig;
const discoverResources = IngestionService.method.discoverResources;

// ========== LIST PROVIDERS ==========

export const createListProvidersQueryKey = (input?: ListProvidersRequest) => {
  return createConnectQueryKey({
    schema: listProviders,
    input,
    cardinality: "finite",
  });
};

export const useListProvidersQuery = ({
  input,
  options = {},
}: {
  input?: ListProvidersRequest;
  options?: UseQueryOptions<typeof listProviders.output, ListProvidersResponse>;
} = {}) => {
  return useQuery<typeof listProviders.input, typeof listProviders.output>(
    listProviders,
    input,
    options,
  );
};

// ========== VALIDATE CONFIG ==========

export const useValidateConfigQuery = ({
  input,
  options = {},
}: {
  input: ValidateConfigRequest;
  options?: UseQueryOptions<typeof validateConfig.output, ValidateConfigResponse>;
}) => {
  return useQuery<typeof validateConfig.input, typeof validateConfig.output>(
    validateConfig,
    input,
    options,
  );
};

// ========== DISCOVER RESOURCES ==========

export const useDiscoverResourcesQuery = ({
  input,
  options = {},
}: {
  input: DiscoverResourcesRequest;
  options?: UseQueryOptions<typeof discoverResources.output, DiscoverResourcesResponse>;
}) => {
  return useQuery<typeof discoverResources.input, typeof discoverResources.output>(
    discoverResources,
    input,
    options,
  );
};
