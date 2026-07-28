import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
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

// ========== LIST CONNECTIONS ==========

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

// ========== GET CONNECTION ==========

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

// ========== MUTATIONS ==========

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
