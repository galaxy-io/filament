import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type {
  GetPipelineVersionRequest,
  GetPipelineVersionResponse,
} from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { createGetPipelineQueryKey, createListPipelinesQueryKey } from "@/api/queries/pipelines";

// ========== GET PIPELINE VERSION ==========

export const createGetPipelineVersionQueryKey = (
  input?: GetPipelineVersionRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipelineVersion,
    input,
    transport,
    cardinality: "finite",
  });
};

export const createGetPipelineVersionQueryOptions = ({
  input,
  transport,
}: {
  input: GetPipelineVersionRequest;
  transport: Transport;
}) => {
  return createQueryOptions(IngestionService.method.getPipelineVersion, input, {
    transport,
  });
};

export const useGetPipelineVersionQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineVersionRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getPipelineVersion.output,
    GetPipelineVersionResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getPipelineVersion.input,
    typeof IngestionService.method.getPipelineVersion.output
  >(IngestionService.method.getPipelineVersion, input, options);
};

// ========== MUTATIONS ==========

export const useCreatePipelineVersionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipelineVersion.input,
    typeof IngestionService.method.createPipelineVersion.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.createPipelineVersion.input,
    typeof IngestionService.method.createPipelineVersion.output
  >(IngestionService.method.createPipelineVersion, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineVersionQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};
