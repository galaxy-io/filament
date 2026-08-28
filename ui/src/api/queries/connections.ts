import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createInfiniteQueryOptions,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useSuspenseInfiniteQuery,
  useSuspenseQuery,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type {
  GetConnectionRequest,
  GetConnectionResponse,
  ListConnectionsRequest,
  ListConnectionsResponse,
} from "@/gen/ingestion/v1/connections_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { createDiscoverResourcesQueryKey } from "@/api/queries/connectors";
import {
  createListSearchInput,
  getNextPageParam,
  INITIAL_PAGE_PARAM,
  type InfiniteQueryInput,
  type ListSearchParams,
  type UseInfiniteQueryOptions,
} from "@/api/utils";

export const createListConnectionsInput = (kind: ConnectorKind, search: ListSearchParams) => ({
  kind,
  ...createListSearchInput(search),
});

export const createListConnectionsQueryKey = (
  input?: ListConnectionsRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listConnections,
    input,
    transport,
    cardinality: undefined,
  });
};

export const createListConnectionsQueryOptions = ({
  input,
  transport,
}: {
  input?: ListConnectionsRequest;
  transport: Transport;
}) => {
  return createQueryOptions(IngestionService.method.listConnections, input, {
    transport,
  });
};

export const useListConnectionsQuery = ({
  input,
  options = {},
}: {
  input?: ListConnectionsRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.listConnections.output,
    ListConnectionsResponse
  >;
} = {}) => {
  return useQuery<
    typeof IngestionService.method.listConnections.input,
    typeof IngestionService.method.listConnections.output
  >(IngestionService.method.listConnections, input, options);
};

export const useSuspenseListConnectionsQuery = ({
  input,
}: {
  input?: ListConnectionsRequest;
} = {}) => {
  return useSuspenseQuery<
    typeof IngestionService.method.listConnections.input,
    typeof IngestionService.method.listConnections.output
  >(IngestionService.method.listConnections, input);
};

export const createListConnectionsInfiniteQueryOptions = ({
  input,
  transport,
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listConnections.input>;
  transport: Transport;
}) => {
  return createInfiniteQueryOptions(
    IngestionService.method.listConnections,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    { transport, pageParamKey: "pagination", getNextPageParam },
  );
};

export const useListConnectionsInfiniteQuery = ({
  input,
  options = {},
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listConnections.input>;
  options?: UseInfiniteQueryOptions<
    typeof IngestionService.method.listConnections.input,
    typeof IngestionService.method.listConnections.output,
    "pagination"
  >;
} = {}) => {
  return useInfiniteQuery<
    typeof IngestionService.method.listConnections.input,
    typeof IngestionService.method.listConnections.output,
    "pagination"
  >(
    IngestionService.method.listConnections,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    { pageParamKey: "pagination", getNextPageParam, ...options },
  );
};

export const useSuspenseListConnectionsInfiniteQuery = ({
  input,
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listConnections.input>;
} = {}) => {
  return useSuspenseInfiniteQuery<
    typeof IngestionService.method.listConnections.input,
    typeof IngestionService.method.listConnections.output,
    "pagination"
  >(
    IngestionService.method.listConnections,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    { pageParamKey: "pagination", getNextPageParam },
  );
};

export const createGetConnectionQueryKey = (
  input?: GetConnectionRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getConnection,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useGetConnectionQuery = ({
  input,
  options = {},
}: {
  input: GetConnectionRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getConnection.output,
    GetConnectionResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getConnection.input,
    typeof IngestionService.method.getConnection.output
  >(IngestionService.method.getConnection, input, options);
};

export const useCreateConnectionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createConnection.input,
    typeof IngestionService.method.createConnection.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.createConnection.input,
    typeof IngestionService.method.createConnection.output
  >(IngestionService.method.createConnection, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListConnectionsQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useUpdateConnectionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updateConnection.input,
    typeof IngestionService.method.updateConnection.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.updateConnection.input,
    typeof IngestionService.method.updateConnection.output
  >(IngestionService.method.updateConnection, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListConnectionsQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetConnectionQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createDiscoverResourcesQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useDeleteConnectionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.deleteConnection.input,
    typeof IngestionService.method.deleteConnection.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.deleteConnection.input,
    typeof IngestionService.method.deleteConnection.output
  >(IngestionService.method.deleteConnection, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListConnectionsQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetConnectionQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};
