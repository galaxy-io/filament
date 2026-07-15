import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  UseMutationOptions,
  UseQueryOptions,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import {
  ListConnectionsResponse,
} from "@/gen/ingestion/v1/connections_pb";
import {
  DiscoverResourcesRequest,
  DiscoverResourcesResponse,
  ListConnectorsRequest,
  ListConnectorsResponse,
  ValidateConfigRequest,
  ValidateConfigResponse,
} from "@/gen/ingestion/v1/providers_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

const listConnections = IngestionService.method.listConnections;
const listConnectors = IngestionService.method.listConnectors;
const validateConfig = IngestionService.method.validateConfig;
const discoverResources = IngestionService.method.discoverResources;

// ========== LIST CONNECTIONS ==========

export const createListConnectionsQueryKey = (input?: { kind?: number }) => {
  return createConnectQueryKey({
    schema: listConnections,
    input,
    cardinality: "finite",
  });
};

export const useListConnectionsQuery = ({
  input,
  options = {},
}: {
  input?: { kind?: number };
  options?: UseQueryOptions<typeof listConnections.output, ListConnectionsResponse>;
} = {}) => {
  return useQuery<typeof listConnections.input, typeof listConnections.output>(
    listConnections,
    input,
    options,
  );
};

const useInvalidateConnections = () => {
  const queryClient = useQueryClient();
  return () =>
    queryClient.invalidateQueries({
      queryKey: createConnectQueryKey({
        schema: listConnections,
        cardinality: "finite",
      }),
    });
};

export const useDeleteConnectionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.deleteConnection.input,
    typeof IngestionService.method.deleteConnection.output
  > = {},
) => {
  const invalidate = useInvalidateConnections();
  return useMutation(IngestionService.method.deleteConnection, {
    ...options,
    onSettled: (...args) => {
      void invalidate();
      return options.onSettled?.(...args);
    },
  });
};

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
