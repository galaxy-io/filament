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
  ValidatePipelineRequest,
  ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import type {
  GetPipelineRequest,
  ListPipelinesRequest,
  ListPipelinesResponse,
} from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

export const createListPipelinesQueryKey = (
  input?: ListPipelinesRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listPipelines,
    input,
    transport,
    cardinality: "finite",
  });
};

export const createListPipelinesQueryOptions = ({
  input,
  transport,
}: {
  input?: ListPipelinesRequest;
  transport: Transport;
}) => {
  return createQueryOptions(IngestionService.method.listPipelines, input, {
    transport,
  });
};

export const useListPipelinesQuery = ({
  input,
  options = {},
}: {
  input?: ListPipelinesRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.listPipelines.output,
    ListPipelinesResponse
  >;
} = {}) => {
  return useQuery<
    typeof IngestionService.method.listPipelines.input,
    typeof IngestionService.method.listPipelines.output
  >(IngestionService.method.listPipelines, input, options);
};

export const useSuspenseListPipelinesQuery = ({ input }: { input?: ListPipelinesRequest } = {}) => {
  return useSuspenseQuery<
    typeof IngestionService.method.listPipelines.input,
    typeof IngestionService.method.listPipelines.output
  >(IngestionService.method.listPipelines, input);
};

export const createGetPipelineQueryKey = (input?: GetPipelineRequest, transport?: Transport) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipeline,
    input,
    transport,
    cardinality: "finite",
  });
};

export const createGetPipelineQueryOptions = ({
  input,
  transport,
}: {
  input: GetPipelineRequest;
  transport: Transport;
}) => {
  return createQueryOptions(IngestionService.method.getPipeline, input, {
    transport,
  });
};

export const useSuspenseGetPipelineQuery = ({ input }: { input: GetPipelineRequest }) => {
  return useSuspenseQuery<
    typeof IngestionService.method.getPipeline.input,
    typeof IngestionService.method.getPipeline.output
  >(IngestionService.method.getPipeline, input);
};

export const useCreatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipeline.input,
    typeof IngestionService.method.createPipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.createPipeline.input,
    typeof IngestionService.method.createPipeline.output
  >(IngestionService.method.createPipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useUpdatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  >(IngestionService.method.updatePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useValidatePipelineQuery = ({
  input,
  options = {},
}: {
  input: ValidatePipelineRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.validatePipeline.output,
    ValidatePipelineResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.validatePipeline.input,
    typeof IngestionService.method.validatePipeline.output
  >(IngestionService.method.validatePipeline, input, options);
};

export const useDeletePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.deletePipeline.input,
    typeof IngestionService.method.deletePipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.deletePipeline.input,
    typeof IngestionService.method.deletePipeline.output
  >(IngestionService.method.deletePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};
