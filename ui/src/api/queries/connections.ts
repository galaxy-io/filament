import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useSuspenseQuery,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type {
  GetConnectionRequest,
  GetConnectionResponse,
  ListConnectionsRequest,
  ListConnectionsResponse,
} from "@/gen/ingestion/v1/connections_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

export const createListConnectionsQueryKey = (
  input?: ListConnectionsRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listConnections,
    input,
    transport,
    cardinality: "finite",
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
      return options.onSettled?.(...args);
    },
  });
};
