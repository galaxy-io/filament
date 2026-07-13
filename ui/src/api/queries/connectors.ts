import {
  createConnectQueryKey,
  useQuery,
  UseQueryOptions,
} from "@connectrpc/connect-query";

import {
  DiscoverResourcesRequest,
  DiscoverResourcesResponse,
  ListConnectorsRequest,
  ListConnectorsResponse,
  ValidateConfigRequest,
  ValidateConfigResponse,
} from "@/gen/ingestion/v1/providers_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

const listConnectors = IngestionService.method.listConnectors;
const validateConfig = IngestionService.method.validateConfig;
const discoverResources = IngestionService.method.discoverResources;

// ========== LIST CONNECTORS ==========

export const createListConnectorsQueryKey = (input?: ListConnectorsRequest) => {
  return createConnectQueryKey({
    schema: listConnectors,
    input,
    cardinality: "finite",
  });
};

export const useListConnectorsQuery = ({
  input,
  options = {},
}: {
  input?: ListConnectorsRequest;
  options?: UseQueryOptions<typeof listConnectors.output, ListConnectorsResponse>;
} = {}) => {
  return useQuery<typeof listConnectors.input, typeof listConnectors.output>(
    listConnectors,
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
