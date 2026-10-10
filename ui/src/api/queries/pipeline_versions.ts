import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type {
  GetPipelineVersionRequest,
  GetPipelineVersionResponse,
} from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { createGetPipelineQueryKey, createListPipelinesQueryKey } from "@/api/queries/pipelines";
import { createListRunsQueryKey } from "@/api/queries/runs";

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
  return useQuery(IngestionService.method.getPipelineVersion, input, options);
};

export const useCreatePipelineVersionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipelineVersion.input,
    typeof IngestionService.method.createPipelineVersion.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.createPipelineVersion, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineVersionQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};
